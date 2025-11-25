package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"debtster-export/internal/clients"
	"debtster-export/internal/domain"
	"debtster-export/internal/repository"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type DebtRepository interface {
	List(ctx context.Context, f repository.DebtsFilter) ([]domain.Debt, error)
}

type ExportStatus struct {
	Key      string    `json:"key"`
	Type     string    `json:"type"`
	UserID   int64     `json:"user_id"`
	Filters  any       `json:"filters"`
	Progress float64   `json:"progress"`
	FileURL  *string   `json:"file_url"`
	Created  time.Time `json:"created_at"`
}

const (
	exportSetKey = "export_ids"
	exportTTL    = 20 * time.Minute
)

type ExportCacheItem struct {
	Key      string
	Type     string
	UserID   int64
	Progress float64
	FileURL  *string
	Created  string
}

type DebtService struct {
	repo        DebtRepository
	redis       *clients.RedisClient
	s3          *clients.S3Client
	ws          *clients.WebSocketClient
	cachePrefix string
}

func NewDebtService(
	repo DebtRepository,
	redis *clients.RedisClient,
	s3 *clients.S3Client,
	ws *clients.WebSocketClient,
) *DebtService {
	prefix := "pkb_database_cache"
	return &DebtService{
		repo:        repo,
		redis:       redis,
		s3:          s3,
		ws:          ws,
		cachePrefix: prefix,
	}
}

func strPtr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func floatPtr(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func timePtr(p *time.Time) string {
	if p == nil {
		return ""
	}
	return p.Format("2006-01-02 15:04:05")
}

type DebtColumn struct {
	Header string
	Value  func(d domain.Debt) any
}

var debtColumns = map[string]DebtColumn{
	"debtor.full_name": {
		Header: "ФИО",
		Value: func(d domain.Debt) any {
			parts := []string{
				strPtr(d.DebtorLastName),
				strPtr(d.DebtorFirstName),
				strPtr(d.DebtorMiddleName),
			}
			return strings.TrimSpace(strings.Join(parts, " "))
		},
	},
	"debtor.iin": {
		Header: "ИИН",
		Value:  func(d domain.Debt) any { return strPtr(d.DebtorIIN) },
	},
	"registry.number": {
		Header: "Номер реестра",
		Value:  func(d domain.Debt) any { return strPtr(d.RegistryNumber) },
	},
	"registry.date": {
		Header: "Дата реестра",
		Value:  func(d domain.Debt) any { return timePtr(d.RegistryDate) },
	},
	"counterparty.name": {
		Header: "Контрагент",
		Value:  func(d domain.Debt) any { return strPtr(d.CounterpartyName) },
	},
	"user.username": {
		Header: "Логин сотрудника",
		Value:  func(d domain.Debt) any { return strPtr(d.UserUsername) },
	},
	"user.departments": {
		Header: "Отдел",
		Value:  func(d domain.Debt) any { return strPtr(d.UserDepartments) },
	},
	"status.name": {
		Header: "Статус",
		Value:  func(d domain.Debt) any { return strPtr(d.StatusName) },
	},
	"start_date": {
		Header: "Дата выдачи займа",
		Value:  func(d domain.Debt) any { return timePtr(d.StartDate) },
	},
	"end_date": {
		Header: "Дата окончания договора",
		Value:  func(d domain.Debt) any { return timePtr(d.EndDate) },
	},
	"filial": {
		Header: "Каким филиалом выдавался кредит",
		Value:  func(d domain.Debt) any { return strPtr(d.Filial) },
	},
	"product_name": {
		Header: "Наименование продукта",
		Value:  func(d domain.Debt) any { return strPtr(d.ProductName) },
	},
	"amount_currency": {
		Header: "Валюта",
		Value:  func(d domain.Debt) any { return strPtr(d.AmountCurrency) },
	},
	"amount_actual_debt": {
		Header: "Актуальный остаток задолженности",
		Value:  func(d domain.Debt) any { return d.AmountActualDebt },
	},
	"amount_purchased_loan": {
		Header: "Сумма выкупленного кредита",
		Value:  func(d domain.Debt) any { return d.AmountPurchasedLoan },
	},
	"init_amount_actual_debt": {
		Header: "Сумма выкупленного долга",
		Value:  func(d domain.Debt) any { return d.InitAmountActualDebt },
	},
	"amount_credit": {
		Header: "Сумма кредита",
		Value:  func(d domain.Debt) any { return d.AmountCredit },
	},
	"amount_main_debt": {
		Header: "Сумма основного долга",
		Value:  func(d domain.Debt) any { return floatPtr(d.AmountMainDebt) },
	},
	"amount_fine": {
		Header: "Пеня",
		Value:  func(d domain.Debt) any { return d.AmountFine },
	},
	"amount_accrual": {
		Header: "Начисленное вознаграждение по Договору займа",
		Value:  func(d domain.Debt) any { return d.AmountAccrual },
	},
	"amount_government_duty": {
		Header: "Гос.пошлина",
		Value:  func(d domain.Debt) any { return d.AmountGovernmentDuty },
	},
	"amount_representation_expenses": {
		Header: "Представительские расходы",
		Value:  func(d domain.Debt) any { return d.AmountRepresentationExp },
	},
	"amount_notary_fees": {
		Header: "Нотариальные расходы",
		Value:  func(d domain.Debt) any { return d.AmountNotaryFees },
	},
	"amount_postage": {
		Header: "Почтовые расходы",
		Value:  func(d domain.Debt) any { return d.AmountPostage },
	},
	"transfer_decision": {
		Header: "Решение о передаче",
		Value:  func(d domain.Debt) any { return strPtr(d.TransferDecision) },
	},
	"presence_solidarity": {
		Header: "Наличие солидарности",
		Value:  func(d domain.Debt) any { return d.PresenceSolidarity },
	},
	"government_duty_paid": {
		Header: "Гос.пошлина оплачена",
		Value:  func(d domain.Debt) any { return d.GovernmentDutyPaid },
	},
	"government_duty_refund": {
		Header: "Возврат гос.пошлины",
		Value:  func(d domain.Debt) any { return d.GovernmentDutyRefund },
	},
	"representation_expenses_paid": {
		Header: "Представительские расходы оплачены",
		Value:  func(d domain.Debt) any { return d.RepresentationExpensesPaid },
	},
	"late_due_date": {
		Header: "Дата вынесения на просрочку",
		Value:  func(d domain.Debt) any { return timePtr(d.LateDueDate) },
	},
	"next_contact": {
		Header: "Дата следующего контакта",
		Value:  func(d domain.Debt) any { return timePtr(d.NextContact) },
	},
	"last_contact": {
		Header: "Последний контакт",
		Value:  func(d domain.Debt) any { return timePtr(d.LastContact) },
	},
	"additional_data": {
		Header: "Дополнительные данные",
		Value: func(d domain.Debt) any {
			if len(d.AdditionalData) == 0 {
				return ""
			}
			return string(d.AdditionalData)
		},
	},
	"number": {
		Header: "Номер договора",
		Value:  func(d domain.Debt) any { return d.Number },
	},
}

func (s *DebtService) saveExportStatus(ctx context.Context, st *ExportStatus) error {
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

func (s *DebtService) toCacheItem(st *ExportStatus) ExportCacheItem {
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

func phpSerializeExportItem(item ExportCacheItem) string {
	phpStr := func(s string) string {
		return fmt.Sprintf(`s:%d:"%s";`, len(s), s)
	}

	var b strings.Builder
	b.WriteString("a:7:{")

	b.WriteString(phpStr("key"))
	b.WriteString(phpStr(item.Key))

	b.WriteString(phpStr("type"))
	b.WriteString(phpStr(item.Type))

	b.WriteString(phpStr("user_id"))
	b.WriteString(fmt.Sprintf("i:%d;", item.UserID))

	b.WriteString(phpStr("filters"))
	b.WriteString("a:0:{}")

	b.WriteString(phpStr("progress"))
	b.WriteString(fmt.Sprintf("d:%.0f;", item.Progress))

	b.WriteString(phpStr("file_url"))
	if item.FileURL == nil || *item.FileURL == "" {
		b.WriteString("N;")
	} else {
		b.WriteString(phpStr(*item.FileURL))
	}

	b.WriteString(phpStr("created_at"))
	b.WriteString(phpStr(item.Created))

	b.WriteString("}")

	return b.String()
}

func (s *DebtService) saveLaravelCache(ctx context.Context, st *ExportStatus) error {
	if s.redis == nil {
		return nil
	}

	cacheKey := s.cachePrefix + st.Key
	item := s.toCacheItem(st)
	serialized := phpSerializeExportItem(item)

	return s.redis.Set(ctx, cacheKey, serialized, exportTTL)
}

func (s *DebtService) StartDebtsExport(
	ctx context.Context,
	selected []string,
	filter repository.DebtsFilter,
	userID int64,
) (string, error) {
	if len(selected) == 0 {
		selected = []string{
			"number",
			"debtor.full_name",
			"amount_actual_debt",
		}
	}

	exportID := fmt.Sprintf("exports:%s", uuid.NewString())
	now := time.Now()

	status := &ExportStatus{
		Key:      exportID,
		Type:     "debts",
		UserID:   userID,
		Filters:  buildDebtsFiltersMap(filter, selected),
		Progress: 0,
		FileURL:  nil,
		Created:  now,
	}

	_ = s.saveExportStatus(ctx, status)
	_ = s.saveLaravelCache(ctx, status)

	go s.runDebtsExport(context.Background(), exportID, selected, filter, userID, now)

	return exportID, nil
}

func (s *DebtService) runDebtsExport(
	ctx context.Context,
	exportID string,
	selected []string,
	filter repository.DebtsFilter,
	userID int64,
	createdAt time.Time,
) {
	status := &ExportStatus{
		Key:      exportID,
		Type:     "debts",
		UserID:   userID,
		Filters:  buildDebtsFiltersMap(filter, selected),
		Progress: 0,
		FileURL:  nil,
		Created:  createdAt,
	}

	debts, err := s.repo.List(ctx, filter)
	if err != nil {
		// можно было бы сохранить ошибку в отдельное поле, если надо
		return
	}

	var cols []DebtColumn
	for _, key := range selected {
		col, ok := debtColumns[key]
		if !ok {
			continue
		}
		cols = append(cols, col)
	}
	if len(cols) == 0 {
		return
	}

	f := excelize.NewFile()
	sheet := "Debts"
	f.SetSheetName(f.GetSheetName(0), sheet)

	_ = f.SetDocProps(&excelize.DocProperties{
		Creator: fmt.Sprintf("user_%d", userID),
	})

	for i, col := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, col.Header)
	}

	total := len(debts)
	if total == 0 {
		// don't set progress to 100 here — file URL is not ready yet.
		// keep progress at 0 and continue to generate/upload the file;
		// final 100 will be set only after successful upload and URL generation.
	} else {
		chunkSize := 1000
		rowIdx := 2

		for i, d := range debts {
			for colIdx, col := range cols {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
				_ = f.SetCellValue(sheet, cell, col.Value(d))
			}
			rowIdx++

			if (i+1)%chunkSize == 0 || i == total-1 {
				raw := float64(i+1) / float64(total) * 100.0
				progress := math.Round(raw)
				// Never report 100% based on row processing — reserve 100% for when file_url is ready
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

	fileName := fmt.Sprintf("debts_%s.xlsx", time.Now().Format("20060102_150405"))

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

func buildDebtsFiltersMap(f repository.DebtsFilter, fields []string) map[string]interface{} {
	m := map[string]interface{}{}
	if f.UserID != nil {
		m["user_id"] = *f.UserID
	} else {
		m["user_id"] = nil
	}
	if f.StatusID != nil {
		m["status_id"] = *f.StatusID
	} else {
		m["status_id"] = nil
	}
	if f.RegistryID != nil {
		m["registry_id"] = *f.RegistryID
	} else {
		m["registry_id"] = nil
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
	m["fields"] = fields
	return m
}
