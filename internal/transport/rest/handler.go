package rest

import (
	"context"
	"debtster-export/internal/repository"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type DebtExporter interface {
	StartDebtsExport(
		rctx context.Context,
		selected []string,
		filter repository.DebtsFilter,
		userID int64,
	) (string, error)
}

type ActionExporter interface {
	StartActionsExport(
		rctx context.Context,
		selected []string,
		filter repository.ActionsFilter,
		userID int64,
	) (string, error)
}

type UserExporter interface {
	StartUsersExport(
		rctx context.Context,
		selected []string,
		userID int64,
	) (string, error)
}

type PaymentExporter interface {
	StartPaymentsExport(ctx context.Context, selected []string, filter repository.PaymentsFilter, userID int64) (string, error)
}

type Handler struct {
	debts      DebtExporter
	users      UserExporter
	actions    ActionExporter
	payments   PaymentExporter
	exportList ExportListService
}

func NewHandler(debts DebtExporter, users UserExporter, actions ActionExporter, payments PaymentExporter, exportList ExportListService) *Handler {
	return &Handler{
		debts:      debts,
		users:      users,
		actions:    actions,
		payments:   payments,
		exportList: exportList,
	}
}

func (h *Handler) InitRouter() *chi.Mux {
	return h.InitRouterWithAuth(nil)
}

func (h *Handler) InitRouterWithAuth(authMiddleware func(http.Handler) http.Handler) *chi.Mux {
	r := chi.NewRouter()

	r.Use(
		middleware.RequestID,
		middleware.RealIP,
		middleware.Logger,
		middleware.Recoverer,
		middleware.Timeout(60*time.Second),
	)

	if authMiddleware != nil {
		r.Use(authMiddleware)
	}

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello from chi!")
	})

	r.Route("/export", func(r chi.Router) {
		r.Get("/", h.listExports)
		r.Get("/{export_id}", h.getExport)
		r.Post("/debts", h.exportDebts)
		r.Post("/users", h.exportUsers)
		r.Post("/actions", h.exportActions)
		r.Post("/payments", h.exportPayments)
	})

	return r
}
