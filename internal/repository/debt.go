package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"debtster-export/internal/domain"
)

type DebtsFilter struct {
	RegistryID     *string
	CounterpartyID *string
	DepartmentID   *int64
	StatusID       *int64
	UserID         *int64
}

type DebtRepository struct {
	db *sql.DB
}

func NewDebtRepository(db *sql.DB) *DebtRepository {
	return &DebtRepository{db: db}
}

func (r *DebtRepository) List(ctx context.Context, f DebtsFilter) ([]domain.Debt, error) {
	baseQuery := `
		SELECT
			d.number,
			d.start_date,
			d.end_date,
			d.filial,
			d.product_name,
			d.amount_currency,
			d.amount_actual_debt,
			d.amount_purchased_loan,
			d.init_amount_actual_debt,
			d.amount_credit,
			d.amount_main_debt,
			d.amount_fine,
			d.amount_accrual,
			d.amount_government_duty,
			d.amount_representation_expenses,
			d.amount_notary_fees,
			d.amount_postage,
			d.transfer_decision,
			d.presence_solidarity,
			d.government_duty_paid,
			d.government_duty_refund,
			d.representation_expenses_paid,
			d.late_due_date,
			d.next_contact,
			d.last_contact,
			d.additional_data,

			rg.number AS registry_number,
			rg.date   AS registry_date,

			u.username       AS user_username,
			ud.departments   AS user_departments,

			ds.name          AS status_name,

			dbt.last_name,
			dbt.first_name,
			dbt.middle_name,
			dbt.iin,

			cp.name          AS counterparty_name
		FROM debts d
		LEFT JOIN registries     rg  ON rg.id  = d.registry_id
		LEFT JOIN users          u   ON u.id   = d.user_id

		LEFT JOIN (
			SELECT
				du.user_id,
				string_agg(dep.display_name, ', ' ORDER BY dep.display_name) AS departments
			FROM department_user du
			JOIN departments dep ON dep.id = du.department_id
			GROUP BY du.user_id
		) ud ON ud.user_id = u.id

		LEFT JOIN debt_statuses  ds  ON ds.id  = d.status_id
		LEFT JOIN debtors        dbt ON dbt.id = d.debtor_id
		LEFT JOIN counterparties cp  ON cp.id  = d.counterparty_id
	`

	where := []string{"1=1"}
	args := []any{}
	i := 1

	if f.RegistryID != nil {
		where = append(where, fmt.Sprintf("d.registry_id = $%d", i))
		args = append(args, *f.RegistryID)
		i++
	}

	if f.CounterpartyID != nil {
		where = append(where, fmt.Sprintf("d.counterparty_id = $%d", i))
		args = append(args, *f.CounterpartyID)
		i++
	}

	if f.StatusID != nil {
		where = append(where, fmt.Sprintf("d.status_id = $%d", i))
		args = append(args, *f.StatusID)
		i++
	}

	if f.UserID != nil {
		where = append(where, fmt.Sprintf("d.user_id = $%d", i))
		args = append(args, *f.UserID)
		i++
	}

	if f.DepartmentID != nil {
		where = append(where, fmt.Sprintf(`
			EXISTS (
				SELECT 1
				FROM department_user du
				WHERE du.user_id = u.id
				  AND du.department_id = $%d
			)`, i))
		args = append(args, *f.DepartmentID)
		i++
	}

	query := baseQuery + " WHERE " + strings.Join(where, " AND ")

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Debt

	for rows.Next() {
		var d domain.Debt

		if err := rows.Scan(
			&d.Number,
			&d.StartDate,
			&d.EndDate,
			&d.Filial,
			&d.ProductName,
			&d.AmountCurrency,
			&d.AmountActualDebt,
			&d.AmountPurchasedLoan,
			&d.InitAmountActualDebt,
			&d.AmountCredit,
			&d.AmountMainDebt,
			&d.AmountFine,
			&d.AmountAccrual,
			&d.AmountGovernmentDuty,
			&d.AmountRepresentationExp,
			&d.AmountNotaryFees,
			&d.AmountPostage,
			&d.TransferDecision,
			&d.PresenceSolidarity,
			&d.GovernmentDutyPaid,
			&d.GovernmentDutyRefund,
			&d.RepresentationExpensesPaid,
			&d.LateDueDate,
			&d.NextContact,
			&d.LastContact,
			&d.AdditionalData,

			&d.RegistryNumber,
			&d.RegistryDate,

			&d.UserUsername,
			&d.UserDepartments,

			&d.StatusName,

			&d.DebtorLastName,
			&d.DebtorFirstName,
			&d.DebtorMiddleName,
			&d.DebtorIIN,

			&d.CounterpartyName,
		); err != nil {
			return nil, err
		}

		result = append(result, d)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
