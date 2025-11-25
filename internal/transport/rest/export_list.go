package rest

import (
	"context"
	"debtster-export/internal/transport/auth"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type ExportListService interface {
	GetExports(ctx context.Context, userID int64) ([]interface{}, error)
	GetExport(ctx context.Context, exportID string, userID int64) (interface{}, error)
}

func (h *Handler) listExports(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserID(r.Context())
	if err != nil {
		ErrorUnauthorized(w, "Unauthorized")
		return
	}

	exports, err := h.exportList.GetExports(r.Context(), userID)
	if err != nil {
		log.Printf("[HTTP] listExports error: %v", err)
		ErrorInternal(w, "failed to get exports")
		return
	}

	Success(w, "", exports)
}

func (h *Handler) getExport(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserID(r.Context())
	if err != nil {
		ErrorUnauthorized(w, "Unauthorized")
		return
	}

	exportIDParam := chi.URLParam(r, "export_id")
	if exportIDParam == "" {
		ErrorBadRequest(w, "export_id is required")
		return
	}
	exportID := "exports:" + exportIDParam

	export, err := h.exportList.GetExport(r.Context(), exportID, userID)
	if err != nil {
		log.Printf("[HTTP] getExport error: %v", err)
		ErrorNotFound(w, "export not found")
		return
	}

	Success(w, "", export)
}
