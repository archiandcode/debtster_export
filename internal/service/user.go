package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"debtster-export/internal/clients"
	"debtster-export/internal/domain"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type UserRepository interface {
	List(ctx context.Context) ([]domain.User, error)
}

type UserService struct {
	repo        UserRepository
	redis       *clients.RedisClient
	s3          *clients.S3Client
	ws          *clients.WebSocketClient
	cachePrefix string
}

func NewUserService(
	repo UserRepository,
	redis *clients.RedisClient,
	s3 *clients.S3Client,
	ws *clients.WebSocketClient,
) *UserService {
	// тот же префикс, что и у DebtService
	prefix := "pkb_database_cache"

	return &UserService{
		repo:        repo,
		redis:       redis,
		s3:          s3,
		ws:          ws,
		cachePrefix: prefix,
	}
}

type UserColumn struct {
	Header string
	Value  func(u domain.User) any
}

var userColumns = map[string]UserColumn{
	"first_name": {
		Header: "Имя",
		Value: func(u domain.User) any {
			return strPtr(u.FirstName)
		},
	},
	"last_name": {
		Header: "Фамилия",
		Value: func(u domain.User) any {
			return strPtr(u.LastName)
		},
	},
	"middle_name": {
		Header: "Отчество",
		Value: func(u domain.User) any {
			return strPtr(u.MiddleName)
		},
	},
	"full_name": {
		Header: "ФИО",
		Value: func(u domain.User) any {
			return fmt.Sprintf("%s %s %s",
				strPtr(u.LastName),
				strPtr(u.FirstName),
				strPtr(u.MiddleName),
			)
		},
	},
	"username": {
		Header: "Логин",
		Value: func(u domain.User) any {
			return strPtr(u.Username)
		},
	},
	"email": {
		Header: "Email",
		Value: func(u domain.User) any {
			return strPtr(u.Email)
		},
	},
	"phone": {
		Header: "Телефон",
		Value: func(u domain.User) any {
			return strPtr(u.Phone)
		},
	},
	"departments": {
		Header: "Отделы",
		Value: func(u domain.User) any {
			return strPtr(u.Departments)
		},
	},
}

// --- helpers для статуса экспорта (аналогичные DebtService) ---

func (s *UserService) saveExportStatus(ctx context.Context, st *ExportStatus) error {
	if s.redis == nil {
		return nil
	}

	data, err := json.Marshal(st)
	if err != nil {
		return err
	}

	if err := s.redis.Set(ctx, st.Key, string(data), exportTTL); err != nil {
		return err
	}

	return s.redis.SAdd(ctx, exportSetKey, st.Key)
}

func (s *UserService) saveLaravelCache(ctx context.Context, st *ExportStatus) error {
	if s.redis == nil {
		return nil
	}

	cacheKey := s.cachePrefix + st.Key
	item := ExportCacheItem{
		Key:      st.Key,
		Type:     st.Type,
		UserID:   st.UserID,
		Progress: st.Progress,
		FileURL:  st.FileURL,
		Created:  st.Created.Format("2006-01-02 15:04:05"),
	}

	serialized := phpSerializeExportItem(item)
	return s.redis.Set(ctx, cacheKey, serialized, exportTTL)
}

// --- публичный метод, который ожидает Handler (как StartDebtsExport) ---

func (s *UserService) StartUsersExport(
	ctx context.Context,
	selected []string,
	userID int64,
) (string, error) {
	if len(selected) == 0 {
		selected = []string{
			"full_name",
			"username",
			"email",
			"departments",
		}
	}

	exportID := fmt.Sprintf("exports:%s", uuid.NewString())
	now := time.Now()

	status := &ExportStatus{
		Key:      exportID,
		Type:     "users",
		UserID:   userID,
		Filters:  buildUsersFiltersMap(selected),
		Progress: 0,
		FileURL:  nil,
		Created:  now,
	}

	_ = s.saveExportStatus(ctx, status)
	_ = s.saveLaravelCache(ctx, status)

	// запускаем фоновую задачу
	go s.runUsersExport(context.Background(), exportID, selected, userID, now)

	return exportID, nil
}

// собственно выполнение экспорта, очень похоже на runDebtsExport
func (s *UserService) runUsersExport(
	ctx context.Context,
	exportID string,
	selected []string,
	userID int64,
	createdAt time.Time,
) {
	status := &ExportStatus{
		Key:      exportID,
		Type:     "users",
		UserID:   userID,
		Filters:  buildUsersFiltersMap(selected),
		Progress: 0,
		FileURL:  nil,
		Created:  createdAt,
	}

	users, err := s.repo.List(ctx)
	if err != nil {
		return
	}

	var cols []UserColumn
	for _, key := range selected {
		col, ok := userColumns[key]
		if !ok {
			continue
		}
		cols = append(cols, col)
	}
	if len(cols) == 0 {
		return
	}

	f := excelize.NewFile()
	sheet := "Users"
	f.SetSheetName(f.GetSheetName(0), sheet)

	_ = f.SetDocProps(&excelize.DocProperties{
		Creator: fmt.Sprintf("user_%d", userID),
	})

	for i, col := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, col.Header)
	}

	total := len(users)
	if total == 0 {
		// don't set progress to 100 here — file URL is not ready yet.
		// keep progress at 0 and continue to generate/upload the file;
		// final 100 will be set only after successful upload and URL generation.
	} else {
		chunkSize := 1000
		rowIdx := 2

		for i, u := range users {
			for colIdx, col := range cols {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
				_ = f.SetCellValue(sheet, cell, col.Value(u))
			}
			rowIdx++

			if (i+1)%chunkSize == 0 || i == total-1 {
				raw := float64(i+1) / float64(total) * 100.0
				progress := math.Round(raw)
				if progress >= 100 {
					progress = 95
				}

				status.Progress = progress
				_ = s.saveExportStatus(ctx, status)
				_ = s.saveLaravelCache(ctx, status)

				if s.ws != nil {
					_ = s.ws.NotifyExportProgress(ctx, userID, exportID, progress, "generating")
				}
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return
	}
	data := buf.Bytes()

	fileName := fmt.Sprintf("users_%s.xlsx", time.Now().Format("20060102_150405"))

	if s.s3 != nil {
		// notify upload phase before starting upload
		status.Progress = 95
		_ = s.saveExportStatus(ctx, status)
		_ = s.saveLaravelCache(ctx, status)
		if s.ws != nil {
			_ = s.ws.NotifyExportProgress(ctx, userID, exportID, 95, "uploading")
		}

		key, err := s.s3.UploadXLSX(ctx, fileName, data)
		if err == nil {
			url, err2 := s.s3.GetTemporaryURL(ctx, key, 48*time.Hour)
			if err2 == nil {
				status.FileURL = &url
				status.Progress = 100

				_ = s.saveExportStatus(ctx, status)
				_ = s.saveLaravelCache(ctx, status)

				if s.ws != nil {
					_ = s.ws.NotifyExportProgress(ctx, userID, exportID, 100, "ready")
					_ = s.ws.NotifyExportComplete(ctx, userID, exportID, url, fileName)
				}
			}
		}
	}
}

// buildUsersFiltersMap возвращает карту с выбранными полями для экспорта пользователей
func buildUsersFiltersMap(fields []string) map[string]interface{} {
	m := map[string]interface{}{}
	m["fields"] = fields
	return m
}

// при желании можно оставить синхронный метод (если где-то нужен)
func (s *UserService) ExportUsersToXLSX(
	ctx context.Context,
	selected []string,
) ([]byte, error) {
	if len(selected) == 0 {
		selected = []string{
			"full_name",
			"username",
			"email",
			"departments",
		}
	}

	users, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	var cols []UserColumn
	for _, key := range selected {
		col, ok := userColumns[key]
		if !ok {
			continue
		}
		cols = append(cols, col)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no valid user columns selected")
	}

	f := excelize.NewFile()
	sheet := "Users"
	f.SetSheetName(f.GetSheetName(0), sheet)

	for i, col := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, col.Header)
	}

	rowIdx := 2
	for _, u := range users {
		for colIdx, col := range cols {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
			_ = f.SetCellValue(sheet, cell, col.Value(u))
		}
		rowIdx++
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
