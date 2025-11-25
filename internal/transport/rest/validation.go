package rest

import (
	"debtster-export/internal/repository"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
)

type ExportRequest struct {
	Fields         []string `json:"fields"`
	RegistryID     *string  `json:"registry_id,omitempty"`
	CounterpartyID *string  `json:"counterparty_id,omitempty"`
	DepartmentID   *string  `json:"department_id,omitempty"`
	StatusID       *int64   `json:"status_id,omitempty"`
	UserID         *int64   `json:"user_id,omitempty"`
}

type rawExportRequest struct {
	Fields         []string    `json:"fields"`
	RegistryID     interface{} `json:"registry_id"`
	CounterpartyID interface{} `json:"counterparty_id"`
	DepartmentID   interface{} `json:"department_id"`
	StatusID       interface{} `json:"status_id"`
	UserID         interface{} `json:"user_id"`
}

func ValidateExportRequest(r *http.Request) (*ExportRequest, error) {
	var raw rawExportRequest

	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil && err != io.EOF {
		return nil, err
	}

	if len(raw.Fields) == 0 {
		return nil, &ValidationError{Field: "fields", Message: "fields is required and must be an array"}
	}

	registryID, err := toStringPtr(raw.RegistryID)
	if err != nil {
		return nil, &ValidationError{Field: "registry_id", Message: "registry_id must be string or empty"}
	}

	counterpartyID, err := toStringPtr(raw.CounterpartyID)
	if err != nil {
		return nil, &ValidationError{Field: "counterparty_id", Message: "counterparty_id must be string or empty"}
	}

	departmentID, err := toStringPtr(raw.DepartmentID)
	if err != nil {
		return nil, &ValidationError{Field: "department_id", Message: "department_id must be string/number or empty"}
	}

	statusID, err := toInt64Ptr(raw.StatusID)
	if err != nil {
		return nil, &ValidationError{Field: "status_id", Message: "status_id must be integer or empty"}
	}

	userID, err := toInt64Ptr(raw.UserID)
	if err != nil {
		return nil, &ValidationError{Field: "user_id", Message: "user_id must be integer or empty"}
	}

	return &ExportRequest{
		Fields:         raw.Fields,
		RegistryID:     registryID,
		CounterpartyID: counterpartyID,
		DepartmentID:   departmentID,
		StatusID:       statusID,
		UserID:         userID,
	}, nil
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type DebtsFilter struct {
	RegistryID     string
	CounterpartyID string
	DepartmentID   string
	StatusID       *int64
	UserID         *int64
}

func (r *ExportRequest) ToDebtsFilter() DebtsFilter {
	f := DebtsFilter{}

	if r.RegistryID != nil && *r.RegistryID != "" {
		f.RegistryID = *r.RegistryID
	}
	if r.CounterpartyID != nil && *r.CounterpartyID != "" {
		f.CounterpartyID = *r.CounterpartyID
	}
	if r.DepartmentID != nil && *r.DepartmentID != "" {
		f.DepartmentID = *r.DepartmentID
	}
	if r.StatusID != nil {
		f.StatusID = r.StatusID
	}
	if r.UserID != nil {
		f.UserID = r.UserID
	}

	return f
}

func toStringPtr(v interface{}) (*string, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case string:
		if t == "" {
			return nil, nil
		}
		return &t, nil
	case float64:
		i := int64(t)
		s := strconv.FormatInt(i, 10)
		return &s, nil
	default:
		return nil, &ValidationError{Message: "invalid type for string field"}
	}
}

func toInt64Ptr(v interface{}) (*int64, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case float64:
		i := int64(t)
		return &i, nil
	case string:
		if t == "" {
			return nil, nil
		}
		i, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return nil, err
		}
		return &i, nil
	default:
		return nil, &ValidationError{Message: "invalid type for int field"}
	}
}

type ActionsExportRequest struct {
	Fields []string `json:"fields"`

	CounterpartyID *string    `json:"-"`
	StatusID       *int64     `json:"-"`
	DebtStatusID   *int64     `json:"-"`
	DepartmentID   *int64     `json:"-"`
	TypeID         *string    `json:"-"`
	UserID         *int64     `json:"-"`
	CreateFrom     *time.Time `json:"-"`
	CreateTo       *time.Time `json:"-"`
	NextFrom       *time.Time `json:"-"`
	NextTo         *time.Time `json:"-"`
}

type rawActionsExportRequest struct {
	Fields []string `json:"fields"`

	CounterpartyID interface{} `json:"counterparty_id"`
	StatusID       interface{} `json:"status_id"`
	DebtStatusID   interface{} `json:"debt_status_id"`
	DepartmentID   interface{} `json:"department_id"`
	TypeID         interface{} `json:"type_id"`
	UserID         interface{} `json:"user_id"`

	CreateStartDate      interface{} `json:"create_start_date"`
	CreateEndDate        interface{} `json:"create_end_date"`
	NextContactStartDate interface{} `json:"next_contact_start_date"`
	NextContactEndDate   interface{} `json:"next_contact_end_date"`
}

func ValidateActionsExportRequest(r *http.Request) (*ActionsExportRequest, error) {
	var raw rawActionsExportRequest

	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil && err != io.EOF {
		return nil, err
	}

	if len(raw.Fields) == 0 {
		return nil, &ValidationError{Field: "fields", Message: "fields is required and must be an array"}
	}

	counterpartyID, err := toStringPtr(raw.CounterpartyID)
	if err != nil {
		return nil, &ValidationError{Field: "counterparty_id", Message: "counterparty_id must be string or empty"}
	}

	statusID, err := toInt64Ptr(raw.StatusID)
	if err != nil {
		return nil, &ValidationError{Field: "status_id", Message: "status_id must be integer or empty"}
	}

	debtStatusID, err := toInt64Ptr(raw.DebtStatusID)
	if err != nil {
		return nil, &ValidationError{Field: "debt_status_id", Message: "debt_status_id must be integer or empty"}
	}

	departmentIDStr, err := toStringPtr(raw.DepartmentID)
	if err != nil {
		return nil, &ValidationError{Field: "department_id", Message: "department_id must be string/number or empty"}
	}
	var departmentID *int64
	if departmentIDStr != nil && *departmentIDStr != "" {
		if v, err := strconv.ParseInt(*departmentIDStr, 10, 64); err == nil {
			departmentID = &v
		} else {
			return nil, &ValidationError{Field: "department_id", Message: "department_id must be integer or empty"}
		}
	}

	typeID, err := toStringPtr(raw.TypeID)
	if err != nil {
		return nil, &ValidationError{Field: "type_id", Message: "type_id must be string or empty"}
	}

	userID, err := toInt64Ptr(raw.UserID)
	if err != nil {
		return nil, &ValidationError{Field: "user_id", Message: "user_id must be integer or empty"}
	}

	createFrom, err := toDatePtr(raw.CreateStartDate)
	if err != nil {
		return nil, &ValidationError{Field: "create_start_date", Message: "create_start_date must be YYYY-MM-DD or empty"}
	}
	createTo, err := toDatePtr(raw.CreateEndDate)
	if err != nil {
		return nil, &ValidationError{Field: "create_end_date", Message: "create_end_date must be YYYY-MM-DD or empty"}
	}

	nextFrom, err := toDatePtr(raw.NextContactStartDate)
	if err != nil {
		return nil, &ValidationError{Field: "next_contact_start_date", Message: "next_contact_start_date must be YYYY-MM-DD or empty"}
	}
	nextTo, err := toDatePtr(raw.NextContactEndDate)
	if err != nil {
		return nil, &ValidationError{Field: "next_contact_end_date", Message: "next_contact_end_date must be YYYY-MM-DD or empty"}
	}

	return &ActionsExportRequest{
		Fields:         raw.Fields,
		CounterpartyID: counterpartyID,
		StatusID:       statusID,
		DebtStatusID:   debtStatusID,
		DepartmentID:   departmentID,
		TypeID:         typeID,
		UserID:         userID,
		CreateFrom:     createFrom,
		CreateTo:       createTo,
		NextFrom:       nextFrom,
		NextTo:         nextTo,
	}, nil
}

func (r *ActionsExportRequest) ToRepositoryFilter() repository.ActionsFilter {
	f := repository.ActionsFilter{
		CounterpartyID:  r.CounterpartyID,
		DebtStatusID:    r.DebtStatusID,
		DepartmentID:    r.DepartmentID,
		TypeID:          r.TypeID,
		UserID:          r.UserID,
		CreatedFrom:     r.CreateFrom,
		CreatedTo:       r.CreateTo,
		NextContactFrom: r.NextFrom,
		NextContactTo:   r.NextTo,
	}
	_ = r.StatusID
	return f
}

func toDatePtr(v interface{}) (*time.Time, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case string:
		if t == "" {
			return nil, nil
		}
		parsed, err := time.Parse("2006-01-02", t)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	default:
		return nil, &ValidationError{Message: "invalid type for date field"}
	}
}
