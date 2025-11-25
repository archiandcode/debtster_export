package rest

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"debtster-export/internal/transport/auth"
)

type UsersExportRequest struct {
	Fields []string `json:"fields"`
}

func (h *Handler) exportUsers(w http.ResponseWriter, r *http.Request) {
	if h.users == nil {
		ErrorInternal(w, "users export not configured")
		return
	}

	var req UsersExportRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		ErrorBadRequest(w, "invalid JSON")
		return
	}

	userID, err := auth.GetUserID(r.Context())
	if err != nil {
		ErrorUnauthorized(w, "Unauthorized")
		return
	}

	exportID, err := h.users.StartUsersExport(r.Context(), req.Fields, userID)
	if err != nil {
		log.Printf("[HTTP] startUsersExport error: %v", err)
		ErrorInternal(w, "failed to start users export")
		return
	}

	SuccessAccepted(w, "Экспорт пользователей поставлен в очередь", map[string]interface{}{
		"export_id": exportID,
	})
}
