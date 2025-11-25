package rest

import (
	"log"
	"net/http"

	"debtster-export/internal/transport/auth"
)

func (h *Handler) exportActions(w http.ResponseWriter, r *http.Request) {
	if h.actions == nil {
		ErrorInternal(w, "actions export not configured")
		return
	}
	req, err := ValidateActionsExportRequest(r)
	if err != nil {
		if _, ok := err.(*ValidationError); ok {
			ErrorBadRequest(w, err.Error())
			return
		}
		ErrorBadRequest(w, "invalid JSON")
		return
	}

	userID, err := auth.GetUserID(r.Context())
	if err != nil {
		ErrorUnauthorized(w, "Unauthorized")
		return
	}

	filter := req.ToRepositoryFilter()

	exportID, err := h.actions.StartActionsExport(r.Context(), req.Fields, filter, userID)
	if err != nil {
		log.Printf("[HTTP] startActionsExport error: %v", err)
		ErrorInternal(w, "failed to start actions export")
		return
	}

	SuccessAccepted(w, "Экспорт действий поставлен в очередь", map[string]interface{}{
		"export_id": exportID,
	})
}
