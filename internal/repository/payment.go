package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"debtster-export/internal/domain"
)

type PaymentsFilter struct {
	Confirmed               *int
	CounterpartyID          *string
	UserID                  *int64
	PeriodImportedStartDate *time.Time
	PeriodImportedEndDate   *time.Time
}

type PaymentRepository struct {
	db *sql.DB
}

func NewPaymentRepository(db *sql.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) List(ctx context.Context, f PaymentsFilter) ([]domain.Payment, error) {
	base := `SELECT p.id, p.debt_id, p.user_id, p.amount, p.amount_after_subtraction, p.amount_government_duty, p.amount_representation_expenses, p.amount_notary_fees, p.amount_postage, p.confirmed, p.payment_date, p.created_at, p.updated_at, p.deleted_at, p.amount_accounts_receivable, p.amount_main_debt, p.amount_accrual, p.amount_fine FROM payments p LEFT JOIN debts d ON d.id = p.debt_id`

	where := []string{"1=1"}
	args := []any{}
	i := 1

	if f.Confirmed != nil {
		where = append(where, fmt.Sprintf("confirmed = $%d", i))
		// Postgres column `confirmed` is boolean; convert incoming int (1/0) to bool
		args = append(args, (*f.Confirmed) == 1)
		i++
	}

	if f.CounterpartyID != nil && *f.CounterpartyID != "" {
		// payments don't directly store counterparty_id — join debts table and filter by d.counterparty_id
		where = append(where, fmt.Sprintf("d.counterparty_id = $%d", i))
		args = append(args, *f.CounterpartyID)
		i++
	}

	if f.UserID != nil {
		where = append(where, fmt.Sprintf("user_id = $%d", i))
		args = append(args, *f.UserID)
		i++
	}

	if f.PeriodImportedStartDate != nil {
		where = append(where, fmt.Sprintf("payment_date >= $%d", i))
		args = append(args, *f.PeriodImportedStartDate)
		i++
	}
	if f.PeriodImportedEndDate != nil {
		where = append(where, fmt.Sprintf("payment_date <= $%d", i))
		args = append(args, *f.PeriodImportedEndDate)
		i++
	}

	query := base + " WHERE " + strings.Join(where, " AND ")

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.Payment
	for rows.Next() {
		var p domain.Payment
		var userID sql.NullInt64
		var paymentDate sql.NullTime
		if err := rows.Scan(
			&p.ID,
			&p.DebtID,
			&userID,
			&p.Amount,
			&p.AmountAfterSubtraction,
			&p.AmountGovernmentDuty,
			&p.AmountRepresentationExpenses,
			&p.AmountNotaryFees,
			&p.AmountPostage,
			&p.Confirmed,
			&paymentDate,
			&p.CreatedAt,
			&p.UpdatedAt,
			&p.DeletedAt,
			&p.AmountAccountsReceivable,
			&p.AmountMainDebt,
			&p.AmountAccrual,
			&p.AmountFine,
		); err != nil {
			return nil, err
		}

		if userID.Valid {
			u := userID.Int64
			p.UserID = &u
		} else {
			p.UserID = nil
		}
		if paymentDate.Valid {
			p.PaymentDate = &paymentDate.Time
		} else {
			p.PaymentDate = nil
		}

		out = append(out, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PaymentRepository) HasMoreThan(ctx context.Context, limit int64, f PaymentsFilter) (bool, error) {
	base := `SELECT COUNT(*) > $1 FROM payments p LEFT JOIN debts d ON d.id = p.debt_id`

	where := []string{"1=1"}
	args := []any{limit}
	i := 2

	if f.Confirmed != nil {
		where = append(where, fmt.Sprintf("confirmed = $%d", i))
		args = append(args, (*f.Confirmed) == 1)
		i++
	}
	if f.CounterpartyID != nil && *f.CounterpartyID != "" {
		where = append(where, fmt.Sprintf("d.counterparty_id = $%d", i))
		args = append(args, *f.CounterpartyID)
		i++
	}
	if f.UserID != nil {
		where = append(where, fmt.Sprintf("user_id = $%d", i))
		args = append(args, *f.UserID)
		i++
	}
	if f.PeriodImportedStartDate != nil {
		where = append(where, fmt.Sprintf("payment_date >= $%d", i))
		args = append(args, *f.PeriodImportedStartDate)
		i++
	}
	if f.PeriodImportedEndDate != nil {
		where = append(where, fmt.Sprintf("payment_date <= $%d", i))
		args = append(args, *f.PeriodImportedEndDate)
		i++
	}

	query := base + " WHERE " + strings.Join(where, " AND ")

	var tooMany bool
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&tooMany); err != nil {
		return false, err
	}
	return tooMany, nil
}
