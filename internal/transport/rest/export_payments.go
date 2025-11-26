package rest

import (
	"debtster-export/internal/repository"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"debtster-export/internal/transport/auth"
)

func (h *Handler) exportPayments(w http.ResponseWriter, r *http.Request) {
	req, err := ValidatePaymentsExportRequest(r)
	if err != nil {
		if _, ok := err.(*ValidationError); ok {
			ErrorBadRequest(w, err.Error())
			return
		}
		ErrorBadRequest(w, "invalid JSON")
		return
	}

	filter := req.ToRepositoryFilter()

	userID, err := auth.GetUserID(r.Context())
	if err != nil {
		ErrorUnauthorized(w, "Unauthorized")
		return
	}

	exportID, err := h.payments.StartPaymentsExport(r.Context(), req.Fields, filter, userID)
	if err != nil {
		log.Printf("[HTTP] startPaymentsExport error: %v", err)
		ErrorInternal(w, "failed to start export")
		return
	}

	SuccessAccepted(w, "Экспорт поставлен в очередь", map[string]interface{}{"export_id": exportID})
}

type PaymentsExportRequest struct {
	Fields              []string   `json:"fields"`
	Confirmed           *int       `json:"confirmed,omitempty"`
	CounterpartyID      *string    `json:"counterparty_id,omitempty"`
	UserID              *int64     `json:"user_id,omitempty"`
	PeriodImportedStart *time.Time `json:"period_imported_start_date,omitempty"`
	PeriodImportedEnd   *time.Time `json:"period_imported_end_date,omitempty"`
}

type rawPaymentsExportRequest struct {
	Fields              []string    `json:"fields"`
	Confirmed           interface{} `json:"confirmed"`
	CounterpartyID      interface{} `json:"counterparty_id"`
	UserID              interface{} `json:"user_id"`
	PeriodImportedStart interface{} `json:"period_imported_start_date"`
	PeriodImportedEnd   interface{} `json:"period_imported_end_date"`
}

// ValidatePaymentsExportRequest parses and validates JSON input for payments export
func ValidatePaymentsExportRequest(r *http.Request) (*PaymentsExportRequest, error) {
	var raw rawPaymentsExportRequest
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil && err != io.EOF {
		return nil, err
	}
	if len(raw.Fields) == 0 {
		return nil, &ValidationError{Field: "fields", Message: "fields is required and must be an array"}
	}

	var confirmed *int
	if raw.Confirmed != nil {
		switch v := raw.Confirmed.(type) {
		case float64:
			i := int(v)
			confirmed = &i
		case string:
			if v != "" {
				if parsed, err := strconv.Atoi(v); err == nil {
					confirmed = &parsed
				} else {
					return nil, &ValidationError{Field: "confirmed", Message: "confirmed must be integer or empty"}
				}
			}
		default:
			return nil, &ValidationError{Field: "confirmed", Message: "confirmed must be integer or empty"}
		}
	}

	counterpartyID, err := toStringPtr(raw.CounterpartyID)
	if err != nil {
		return nil, &ValidationError{Field: "counterparty_id", Message: "counterparty_id must be string or empty"}
	}

	userID, err := toInt64Ptr(raw.UserID)
	if err != nil {
		return nil, &ValidationError{Field: "user_id", Message: "user_id must be integer or empty"}
	}

	// parse dates (YYYY-MM-DD) if provided
	var startDate *time.Time
	var endDate *time.Time
	if raw.PeriodImportedStart != nil {
		switch v := raw.PeriodImportedStart.(type) {
		case string:
			if v != "" {
				parsed, err := time.Parse("2006-01-02", v)
				if err != nil {
					return nil, &ValidationError{Field: "period_imported_start_date", Message: "must be YYYY-MM-DD or empty"}
				}
				startDate = &parsed
			}
		default:
			return nil, &ValidationError{Field: "period_imported_start_date", Message: "must be YYYY-MM-DD or empty"}
		}
	}
	if raw.PeriodImportedEnd != nil {
		switch v := raw.PeriodImportedEnd.(type) {
		case string:
			if v != "" {
				parsed, err := time.Parse("2006-01-02", v)
				if err != nil {
					return nil, &ValidationError{Field: "period_imported_end_date", Message: "must be YYYY-MM-DD or empty"}
				}
				endDate = &parsed
			}
		default:
			return nil, &ValidationError{Field: "period_imported_end_date", Message: "must be YYYY-MM-DD or empty"}
		}
	}

	return &PaymentsExportRequest{
		Fields:              raw.Fields,
		Confirmed:           confirmed,
		CounterpartyID:      counterpartyID,
		UserID:              userID,
		PeriodImportedStart: startDate,
		PeriodImportedEnd:   endDate,
	}, nil
}

func (r *PaymentsExportRequest) ToRepositoryFilter() repository.PaymentsFilter {
	rf := repository.PaymentsFilter{}
	if r.Confirmed != nil {
		rf.Confirmed = r.Confirmed
	}
	if r.CounterpartyID != nil {
		rf.CounterpartyID = r.CounterpartyID
	}
	if r.UserID != nil {
		rf.UserID = r.UserID
	}
	if r.PeriodImportedStart != nil {
		rf.PeriodImportedStartDate = r.PeriodImportedStart
	}
	if r.PeriodImportedEnd != nil {
		rf.PeriodImportedEndDate = r.PeriodImportedEnd
	}
	return rf
}
