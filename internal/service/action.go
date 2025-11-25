package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"debtster-export/internal/clients"
	"debtster-export/internal/domain"
	"debtster-export/internal/repository"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type ActionRepository interface {
	List(ctx context.Context, f repository.ActionsFilter) ([]domain.Action, error)
	HasMoreThan(ctx context.Context, limit int64, f repository.ActionsFilter) (bool, error)
}

type ActionService struct {
	repo        ActionRepository
	redis       *clients.RedisClient
	s3          *clients.S3Client
	ws          *clients.WebSocketClient
	cachePrefix string
}

func NewActionService(
	repo ActionRepository,
	redis *clients.RedisClient,
	s3 *clients.S3Client,
	ws *clients.WebSocketClient,
) *ActionService {
	return &ActionService{
		repo:        repo,
		redis:       redis,
		s3:          s3,
		ws:          ws,
		cachePrefix: "pkb_database_cache",
	}
}

type ActionColumn struct {
	Header string
	Value  func(a domain.Action) any
}

var actionTypeDisplay = map[string]string{
	"incoming_call": "Входящий звонок",
	"outgoing_call": "Исходящий звонок",
}

var actionColumns = map[string]ActionColumn{
	"debt_id": {
		Header: "ID долга",
		Value: func(a domain.Action) any {
			return a.DebtID
		},
	},
	"user_id": {
		Header: "ID пользователя",
		Value: func(a domain.Action) any {
			return a.UserID
		},
	},

	"debt.number": {
		Header: "Номер долга",
		Value: func(a domain.Action) any {
			if a.DebtNumber == nil {
				return ""
			}
			return *a.DebtNumber
		},
	},
	"debt.counterparty.name": {
		Header: "Контрагент",
		Value: func(a domain.Action) any {
			if a.CounterpartyName == nil {
				return ""
			}
			return *a.CounterpartyName
		},
	},

	"debtStatus.name": {
		Header: "Статус долга",
		Value: func(a domain.Action) any {
			if a.DebtStatusName == nil {
				return ""
			}
			return *a.DebtStatusName
		},
	},
	"debt_status_id": {
		Header: "ID статуса долга",
		Value: func(a domain.Action) any {
			return a.DebtStatusID
		},
	},

	"actionType.name": {
		Header: "Тип действия",
		Value: func(a domain.Action) any {
			if title, ok := actionTypeDisplay[a.Type]; ok {
				return title
			}
			return a.Type
		},
	},

	"user.first_name": {
		Header: "Имя пользователя",
		Value: func(a domain.Action) any {
			if a.UserFirstName == nil {
				return ""
			}
			return *a.UserFirstName
		},
	},
	"user.last_name": {
		Header: "Фамилия пользователя",
		Value: func(a domain.Action) any {
			if a.UserLastName == nil {
				return ""
			}
			return *a.UserLastName
		},
	},
	"user.middle_name": {
		Header: "Отчество пользователя",
		Value: func(a domain.Action) any {
			if a.UserMiddleName == nil {
				return ""
			}
			return *a.UserMiddleName
		},
	},
	"user.full_name": {
		Header: "ФИО пользователя",
		Value: func(a domain.Action) any {
			if a.UserFullName != nil && *a.UserFullName != "" {
				return *a.UserFullName
			}
			parts := []string{}
			if a.UserLastName != nil {
				parts = append(parts, *a.UserLastName)
			}
			if a.UserFirstName != nil {
				parts = append(parts, *a.UserFirstName)
			}
			if a.UserMiddleName != nil {
				parts = append(parts, *a.UserMiddleName)
			}
			if len(parts) == 0 {
				return ""
			}
			return strings.Join(parts, " ")
		},
	},
	"user.departments": {
		Header: "Отделы пользователя",
		Value: func(a domain.Action) any {
			if a.UserDepartments == nil {
				return ""
			}
			return *a.UserDepartments
		},
	},

	"debtor.first_name": {
		Header: "Имя должника",
		Value: func(a domain.Action) any {
			if a.DebtorFirstName == nil {
				return ""
			}
			return *a.DebtorFirstName
		},
	},
	"debtor.last_name": {
		Header: "Фамилия должника",
		Value: func(a domain.Action) any {
			if a.DebtorLastName == nil {
				return ""
			}
			return *a.DebtorLastName
		},
	},
	"debtor.middle_name": {
		Header: "Отчество должника",
		Value: func(a domain.Action) any {
			if a.DebtorMiddleName == nil {
				return ""
			}
			return *a.DebtorMiddleName
		},
	},

	"payload": {
		Header: "Payload (JSON)",
		Value: func(a domain.Action) any {
			if len(a.Payload) == 0 {
				return ""
			}
			return string(a.Payload)
		},
	},
	"payload.date_promised_payment": {
		Header: "Дата обещанного платежа",
		Value: func(a domain.Action) any {
			if a.PayloadDatePromisedPayment == nil {
				return ""
			}
			return *a.PayloadDatePromisedPayment
		},
	},
	"payload.amount_promised_payment": {
		Header: "Сумма обещанного платежа",
		Value: func(a domain.Action) any {
			if a.PayloadAmountPromisedPayment == nil {
				return ""
			}
			return *a.PayloadAmountPromisedPayment
		},
	},

	"next_contact": {
		Header: "Следующий контакт",
		Value: func(a domain.Action) any {
			return timePtr(a.NextContact)
		},
	},
	"type": {
		Header: "Тип (raw)",
		Value: func(a domain.Action) any {
			return a.Type
		},
	},
	"comment": {
		Header: "Комментарий",
		Value: func(a domain.Action) any {
			return a.Comment
		},
	},
	"created_at": {
		Header: "Создано",
		Value: func(a domain.Action) any {
			return timePtr(a.CreatedAt)
		},
	},
	"updated_at": {
		Header: "Обновлено",
		Value: func(a domain.Action) any {
			return timePtr(a.UpdatedAt)
		},
	},
	"deleted_at": {
		Header: "Удалено",
		Value: func(a domain.Action) any {
			return timePtr(a.DeletedAt)
		},
	},
}

const maxActionsForExport = 500_000

func (s *ActionService) saveExportStatus(ctx context.Context, st *ExportStatus) error {
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

func (s *ActionService) toCacheItem(st *ExportStatus) ExportCacheItem {
	created := st.Created.Format("2006-01-02 15:04:05")
	return ExportCacheItem{
		Key:      st.Key,
		Type:     st.Type,
		UserID:   st.UserID,
		Progress: st.Progress,
		FileURL:  st.FileURL,
		Created:  created,
	}
}

func (s *ActionService) saveLaravelCache(ctx context.Context, st *ExportStatus) error {
	if s.redis == nil {
		return nil
	}

	cacheKey := s.cachePrefix + st.Key
	item := s.toCacheItem(st)
	serialized := phpSerializeExportItem(item)

	return s.redis.Set(ctx, cacheKey, serialized, exportTTL)
}

func (s *ActionService) StartActionsExport(
	ctx context.Context,
	selected []string,
	filter repository.ActionsFilter,
	userID int64,
) (string, error) {
	if len(selected) == 0 {
		selected = []string{
			"debt.number",
			"actionType.name",
			"user.full_name",
			"comment",
			"created_at",
		}
	}

	tooMany, err := s.repo.HasMoreThan(ctx, maxActionsForExport, filter)
	if err != nil {
		return "", err
	}
	if tooMany {
		return "", fmt.Errorf("слишком много действий для экспорта (больше %d записей)", maxActionsForExport)
	}

	exportID := fmt.Sprintf("exports:%s", uuid.NewString())
	now := time.Now()

	status := &ExportStatus{
		Key:      exportID,
		Type:     "actions",
		UserID:   userID,
		Filters:  buildActionsFiltersMap(filter, selected),
		Progress: 0,
		FileURL:  nil,
		Created:  now,
	}

	_ = s.saveExportStatus(ctx, status)
	_ = s.saveLaravelCache(ctx, status)

	go s.runActionsExport(context.Background(), exportID, selected, filter, userID, now)

	return exportID, nil
}

func (s *ActionService) runActionsExport(
	ctx context.Context,
	exportID string,
	selected []string,
	filter repository.ActionsFilter,
	userID int64,
	createdAt time.Time,
) {
	status := &ExportStatus{
		Key:      exportID,
		Type:     "actions",
		UserID:   userID,
		Filters:  buildActionsFiltersMap(filter, selected),
		Progress: 0,
		FileURL:  nil,
		Created:  createdAt,
	}

	actions, err := s.repo.List(ctx, filter)
	if err != nil {
		return
	}

	var cols []ActionColumn
	for _, key := range selected {
		col, ok := actionColumns[key]
		if !ok {
			continue
		}
		cols = append(cols, col)
	}
	if len(cols) == 0 {
		return
	}

	f := excelize.NewFile()
	sheet := "Actions"
	f.SetSheetName(f.GetSheetName(0), sheet)

	_ = f.SetDocProps(&excelize.DocProperties{
		Creator: fmt.Sprintf("user_%d", userID),
	})

	for i, col := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, col.Header)
	}

	rowIdx := 2
	for _, a := range actions {
		for colIdx, col := range cols {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
			_ = f.SetCellValue(sheet, cell, col.Value(a))
		}
		rowIdx++
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return
	}
	data := buf.Bytes()

	fileName := fmt.Sprintf("actions_%s.xlsx", time.Now().Format("20060102_150405"))

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

func buildActionsFiltersMap(f repository.ActionsFilter, fields []string) map[string]interface{} {
	m := map[string]interface{}{}
	if f.UserID != nil {
		m["user_id"] = *f.UserID
	} else {
		m["user_id"] = nil
	}
	if f.DebtStatusID != nil {
		m["debt_status_id"] = *f.DebtStatusID
	} else {
		m["debt_status_id"] = nil
	}
	if f.CounterpartyID != nil {
		m["counterparty_id"] = *f.CounterpartyID
	} else {
		m["counterparty_id"] = nil
	}
	if f.DepartmentID != nil {
		m["department_id"] = *f.DepartmentID
	} else {
		m["department_id"] = nil
	}
	if f.TypeID != nil {
		m["type_id"] = *f.TypeID
	} else {
		m["type_id"] = nil
	}
	if f.CreatedFrom != nil {
		m["create_start_date"] = f.CreatedFrom.Format("2006-01-02")
	} else {
		m["create_start_date"] = nil
	}
	if f.CreatedTo != nil {
		m["create_end_date"] = f.CreatedTo.Format("2006-01-02")
	} else {
		m["create_end_date"] = nil
	}
	if f.NextContactFrom != nil {
		m["next_contact_start_date"] = f.NextContactFrom.Format("2006-01-02")
	} else {
		m["next_contact_start_date"] = nil
	}
	if f.NextContactTo != nil {
		m["next_contact_end_date"] = f.NextContactTo.Format("2006-01-02")
	} else {
		m["next_contact_end_date"] = nil
	}
	m["fields"] = fields
	return m
}
