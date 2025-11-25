package rest

import (
	"debtster-export/internal/repository"
	"log"
	"net/http"
	"strconv"

	"debtster-export/internal/transport/auth"
)

func (h *Handler) exportDebts(w http.ResponseWriter, r *http.Request) {
	req, err := ValidateExportRequest(r)
	if err != nil {
		if _, ok := err.(*ValidationError); ok {
			ErrorBadRequest(w, err.Error())
			return
		}
		ErrorBadRequest(w, "invalid JSON")
		return
	}

	//userID, err := 1, nil
	//if err != nil {
	//	ErrorUnauthorized(w, "Unauthorized")
	//	return
	//}

	filter := req.ToDebtsFilter().ToRepositoryFilter()

	userID, err := auth.GetUserID(r.Context())
	if err != nil {
		ErrorUnauthorized(w, "Unauthorized")
		return
	}

	exportID, err := h.debts.StartDebtsExport(r.Context(), req.Fields, filter, userID)
	if err != nil {
		log.Printf("[HTTP] startDebtsExport error: %v", err)
		ErrorInternal(w, "failed to start export")
		return
	}

	SuccessAccepted(w, "Экспорт поставлен в очередь", map[string]interface{}{
		"export_id": exportID,
	})
}

func (f DebtsFilter) ToRepositoryFilter() repository.DebtsFilter {
	rf := repository.DebtsFilter{}

	if f.RegistryID != "" {
		rf.RegistryID = &f.RegistryID
	}
	if f.CounterpartyID != "" {
		rf.CounterpartyID = &f.CounterpartyID
	}
	if f.DepartmentID != "" {
		if id, err := strconv.ParseInt(f.DepartmentID, 10, 64); err == nil {
			rf.DepartmentID = &id
		}
	}
	if f.UserID != nil {
		rf.UserID = f.UserID
	}
	if f.StatusID != nil && *f.StatusID != 0 {
		rf.StatusID = f.StatusID
	}

	return rf
}
