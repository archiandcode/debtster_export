package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"debtster-export/internal/clients"
	"debtster-export/internal/domain"
	"debtster-export/internal/repository"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type PaymentRepository interface {
	List(ctx context.Context, f repository.PaymentsFilter) ([]domain.Payment, error)
	HasMoreThan(ctx context.Context, limit int64, f repository.PaymentsFilter) (bool, error)
}

type PaymentColumn struct {
	Header string
	Value  func(p domain.Payment) any
}

var paymentColumns = map[string]PaymentColumn{
	"id":      {Header: "ID", Value: func(p domain.Payment) any { return p.ID }},
	"debt_id": {Header: "ID долга", Value: func(p domain.Payment) any { return p.DebtID }},
	"user_id": {Header: "ID пользователя", Value: func(p domain.Payment) any {
		if p.UserID == nil {
			return ""
		}
		return *p.UserID
	}},
	"confirmed":                      {Header: "Подтвержено", Value: func(p domain.Payment) any { return p.Confirmed }},
	"amount":                         {Header: "Сумма", Value: func(p domain.Payment) any { return p.Amount }},
	"amount_after_subtraction":       {Header: "Сумма после вычета", Value: func(p domain.Payment) any { return p.AmountAfterSubtraction }},
	"amount_government_duty":         {Header: "Госпошлина", Value: func(p domain.Payment) any { return p.AmountGovernmentDuty }},
	"amount_representation_expenses": {Header: "Представительские расходы", Value: func(p domain.Payment) any { return p.AmountRepresentationExpenses }},
	"amount_notary_fees":             {Header: "Нотариальные расходы", Value: func(p domain.Payment) any { return p.AmountNotaryFees }},
	"amount_postage":                 {Header: "Почтовые расходы", Value: func(p domain.Payment) any { return p.AmountPostage }},
	"amount_accounts_receivable":     {Header: "Дебиторская задолженность", Value: func(p domain.Payment) any { return p.AmountAccountsReceivable }},
	"amount_main_debt":               {Header: "Основной долг", Value: func(p domain.Payment) any { return p.AmountMainDebt }},
	"amount_accrual":                 {Header: "Начисления", Value: func(p domain.Payment) any { return p.AmountAccrual }},
	"amount_fine":                    {Header: "Пени", Value: func(p domain.Payment) any { return p.AmountFine }},
	"payment_date":                   {Header: "Дата платежа", Value: func(p domain.Payment) any { return timePtr(p.PaymentDate) }},
	"created_at":                     {Header: "Создано", Value: func(p domain.Payment) any { return timePtr(p.CreatedAt) }},
	"updated_at":                     {Header: "Обновлено", Value: func(p domain.Payment) any { return timePtr(p.UpdatedAt) }},
	"deleted_at":                     {Header: "Удалено", Value: func(p domain.Payment) any { return timePtr(p.DeletedAt) }},
}

const maxPaymentsForExport = 500_000

type PaymentService struct {
	repo        PaymentRepository
	redis       *clients.RedisClient
	s3          *clients.StorageClient
	ws          *clients.WebSocketClient
	cachePrefix string
}

func NewPaymentService(repo PaymentRepository, redis *clients.RedisClient, s3 *clients.StorageClient, ws *clients.WebSocketClient) *PaymentService {
	return &PaymentService{repo: repo, redis: redis, s3: s3, ws: ws, cachePrefix: "pkb_database_cache"}
}

func (s *PaymentService) saveExportStatus(ctx context.Context, st *ExportStatus) error {
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

func (s *PaymentService) saveLaravelCache(ctx context.Context, st *ExportStatus) error {
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
		Error:    st.Error,
		Created:  st.Created.Format("2006-01-02 15:04:05"),
	}
	serialized := phpSerializeExportItem(item)
	return s.redis.Set(ctx, cacheKey, serialized, exportTTL)
}

func (s *PaymentService) StartPaymentsExport(ctx context.Context, selected []string, filter repository.PaymentsFilter, userID int64) (string, error) {
	if len(selected) == 0 {
		selected = []string{"payment_date", "id", "debt_id", "user_id", "confirmed", "amount", "amount_after_subtraction", "amount_government_duty", "amount_representation_expenses", "amount_notary_fees", "amount_postage", "amount_accounts_receivable", "amount_main_debt", "amount_accrual", "amount_fine", "created_at", "updated_at", "deleted_at"}
	}

	tooMany, err := s.repo.HasMoreThan(ctx, maxPaymentsForExport, filter)
	if err != nil {
		return "", err
	}
	if tooMany {
		return "", fmt.Errorf("слишком много платежей для экспорта (больше %d записей)", maxPaymentsForExport)
	}

	exportID := fmt.Sprintf("exports:%s", uuid.NewString())
	now := time.Now()

	status := &ExportStatus{
		Key:      exportID,
		Type:     "payments",
		UserID:   userID,
		Filters:  buildPaymentsFiltersMap(filter, selected),
		Progress: 0,
		FileURL:  nil,
		Created:  now,
	}

	_ = s.saveExportStatus(ctx, status)
	_ = s.saveLaravelCache(ctx, status)

	go s.runPaymentsExport(context.Background(), exportID, selected, filter, userID, now)

	return exportID, nil
}

func (s *PaymentService) runPaymentsExport(ctx context.Context, exportID string, selected []string, filter repository.PaymentsFilter, userID int64, createdAt time.Time) {
	status := &ExportStatus{
		Key:      exportID,
		Type:     "payments",
		UserID:   userID,
		Filters:  buildPaymentsFiltersMap(filter, selected),
		Progress: 0,
		FileURL:  nil,
		Created:  createdAt,
	}

	payments, err := s.repo.List(ctx, filter)
	if err != nil {
		return
	}

	var cols []PaymentColumn
	for _, key := range selected {
		col, ok := paymentColumns[key]
		if !ok {
			continue
		}
		cols = append(cols, col)
	}
	if len(cols) == 0 {
		return
	}

	f := excelize.NewFile()
	sheet := "Payments"
	f.SetSheetName(f.GetSheetName(0), sheet)

	_ = f.SetDocProps(&excelize.DocProperties{Creator: fmt.Sprintf("user_%d", userID)})

	for i, col := range cols {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, col.Header)
	}

	total := len(payments)
	rowIdx := 2
	if total > 0 {
		chunkSize := 1000
		for i, p := range payments {
			for colIdx, col := range cols {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
				_ = f.SetCellValue(sheet, cell, col.Value(p))
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
	} else {
		for _, p := range payments {
			for colIdx, col := range cols {
				cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx)
				_ = f.SetCellValue(sheet, cell, col.Value(p))
			}
			rowIdx++
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return
	}
	data := buf.Bytes()

	fileName := fmt.Sprintf("payments_%s.xlsx", time.Now().Format("20060102_150405"))

	if s.s3 != nil {
		status.Progress = 95
		_ = s.saveExportStatus(ctx, status)
		_ = s.saveLaravelCache(ctx, status)
		if s.ws != nil {
			_ = s.ws.NotifyExportProgress(ctx, userID, exportID, 95, "uploading")
		}

		savedName, err := s.s3.Save(ctx, fileName, data)
		if err != nil {
			errStr := fmt.Sprintf("save export failed: %v", err)
			log.Printf("export %s: %s", exportID, errStr)
			status.Error = &errStr
			status.Progress = 100
			_ = s.saveExportStatus(ctx, status)
			_ = s.saveLaravelCache(ctx, status)
			if s.ws != nil {
				_ = s.ws.NotifyExportFailed(ctx, userID, exportID, errStr)
			}
		} else {
			url := s.s3.GetURL(savedName)
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

func buildPaymentsFiltersMap(f repository.PaymentsFilter, fields []string) map[string]interface{} {
	m := map[string]interface{}{}
	if f.Confirmed != nil {
		m["confirmed"] = *f.Confirmed
	} else {
		m["confirmed"] = nil
	}
	if f.CounterpartyID != nil {
		m["counterparty_id"] = *f.CounterpartyID
	} else {
		m["counterparty_id"] = nil
	}
	if f.UserID != nil {
		m["user_id"] = *f.UserID
	} else {
		m["user_id"] = nil
	}
	if f.PeriodImportedStartDate != nil {
		m["period_imported_start_date"] = f.PeriodImportedStartDate.Format("2006-01-02")
	} else {
		m["period_imported_start_date"] = nil
	}
	if f.PeriodImportedEndDate != nil {
		m["period_imported_end_date"] = f.PeriodImportedEndDate.Format("2006-01-02")
	} else {
		m["period_imported_end_date"] = nil
	}
	m["fields"] = fields
	return m
}
