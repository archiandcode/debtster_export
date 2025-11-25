package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"debtster-export/internal/domain"
)

type ActionsFilter struct {
	CounterpartyID  *string
	DebtStatusID    *int64
	DepartmentID    *int64
	TypeID          *string
	UserID          *int64
	CreatedFrom     *time.Time
	CreatedTo       *time.Time
	NextContactFrom *time.Time
	NextContactTo   *time.Time
}

type ActionRepository struct {
	db *sql.DB
}

func NewActionRepository(db *sql.DB) *ActionRepository {
	return &ActionRepository{
		db: db,
	}
}

func buildActionsWhere(f ActionsFilter, startIndex int, base []string, args []any) (string, []any) {
	where := append([]string{}, base...)
	i := startIndex

	if f.CounterpartyID != nil {
		where = append(where, "d.counterparty_id = $"+strconv.Itoa(i))
		args = append(args, *f.CounterpartyID)
		i++
	}

	if f.DebtStatusID != nil {
		where = append(where, "a.debt_status_id = $"+strconv.Itoa(i))
		args = append(args, *f.DebtStatusID)
		i++
	}

	if f.UserID != nil {
		where = append(where, "a.user_id = $"+strconv.Itoa(i))
		args = append(args, *f.UserID)
		i++
	}

	if f.TypeID != nil && *f.TypeID != "" {
		where = append(where, "a.type = $"+strconv.Itoa(i))
		args = append(args, *f.TypeID)
		i++
	}

	if f.DepartmentID != nil {
		where = append(where, `
			EXISTS (
				SELECT 1
				FROM department_user du
				WHERE du.user_id = u.id
				  AND du.department_id = $`+strconv.Itoa(i)+`
			)`)
		args = append(args, *f.DepartmentID)
		i++
	}

	if f.CreatedFrom != nil {
		where = append(where, "a.created_at >= $"+strconv.Itoa(i))
		args = append(args, *f.CreatedFrom)
		i++
	}
	if f.CreatedTo != nil {
		where = append(where, "a.created_at <= $"+strconv.Itoa(i))
		args = append(args, *f.CreatedTo)
		i++
	}

	if f.NextContactFrom != nil {
		where = append(where, "a.next_contact >= $"+strconv.Itoa(i))
		args = append(args, *f.NextContactFrom)
		i++
	}
	if f.NextContactTo != nil {
		where = append(where, "a.next_contact <= $"+strconv.Itoa(i))
		args = append(args, *f.NextContactTo)
		i++
	}

	return strings.Join(where, " AND "), args
}

func (r *ActionRepository) List(ctx context.Context, f ActionsFilter) ([]domain.Action, error) {
	baseQuery := `
		SELECT
			a.debt_id,
			a.user_id,
			a.debt_status_id,
			a.next_contact,
			a.type,
			a.comment,
			a.payload,
			a.created_at,
			a.updated_at,
			a.deleted_at,

			d.number AS debt_number,

			cp.name AS counterparty_name,

			ds.name AS debt_status_name,

			u.first_name  AS user_first_name,
			u.last_name   AS user_last_name,
			u.middle_name AS user_middle_name,

			ud.departments AS user_departments,

			dbt.first_name  AS debtor_first_name,
			dbt.last_name   AS debtor_last_name,
			dbt.middle_name AS debtor_middle_name
		FROM actions a
		LEFT JOIN debts d
			ON d.id = a.debt_id
		LEFT JOIN counterparties cp
			ON cp.id = d.counterparty_id
		LEFT JOIN debt_statuses ds
			ON ds.id = a.debt_status_id
		LEFT JOIN users u
			ON u.id = a.user_id
		LEFT JOIN (
			SELECT
				du.user_id,
				string_agg(dep.display_name, ', ' ORDER BY dep.display_name) AS departments
			FROM department_user du
			JOIN departments dep ON dep.id = du.department_id
			GROUP BY du.user_id
		) ud ON ud.user_id = u.id
		LEFT JOIN debtors dbt
			ON dbt.id = d.debtor_id
	`

	baseWhere := []string{"a.deleted_at IS NULL"}
	args := []any{}

	whereClause, args := buildActionsWhere(f, 1, baseWhere, args)
	query := baseQuery + " WHERE " + whereClause

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.Action

	for rows.Next() {
		var a domain.Action
		var rawPayload []byte

		if err := rows.Scan(
			&a.DebtID,
			&a.UserID,
			&a.DebtStatusID,
			&a.NextContact,
			&a.Type,
			&a.Comment,
			&rawPayload,
			&a.CreatedAt,
			&a.UpdatedAt,
			&a.DeletedAt,

			&a.DebtNumber,
			&a.CounterpartyName,

			&a.DebtStatusName,

			&a.UserFirstName,
			&a.UserLastName,
			&a.UserMiddleName,

			&a.UserDepartments,

			&a.DebtorFirstName,
			&a.DebtorLastName,
			&a.DebtorMiddleName,
		); err != nil {
			return nil, err
		}

		a.Payload = rawPayload

		if len(rawPayload) > 0 {
			var payload map[string]any
			if err := json.Unmarshal(rawPayload, &payload); err == nil {
				if v, ok := payload["date_promised_payment"].(string); ok && v != "" {
					a.PayloadDatePromisedPayment = &v
				}

				if val, ok := payload["amount_promised_payment"]; ok {
					switch vv := val.(type) {
					case float64:
						a.PayloadAmountPromisedPayment = &vv
					case int:
						f := float64(vv)
						a.PayloadAmountPromisedPayment = &f
					case int64:
						f := float64(vv)
						a.PayloadAmountPromisedPayment = &f
					case string:
						if num, err := strconv.ParseFloat(vv, 64); err == nil {
							a.PayloadAmountPromisedPayment = &num
						}
					}
				}
			}
		}

		if a.UserLastName != nil || a.UserFirstName != nil || a.UserMiddleName != nil {
			full := strings.TrimSpace(
				strings.TrimSpace(strOrEmpty(a.UserLastName)) + " " +
					strings.TrimSpace(strOrEmpty(a.UserFirstName)) + " " +
					strings.TrimSpace(strOrEmpty(a.UserMiddleName)),
			)
			if full != "" {
				a.UserFullName = &full
			}
		}

		result = append(result, a)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *ActionRepository) HasMoreThan(ctx context.Context, limit int64, f ActionsFilter) (bool, error) {
	baseQuery := `
		SELECT COUNT(*) > $1
		FROM actions a
		LEFT JOIN debts d
			ON d.id = a.debt_id
		LEFT JOIN users u
			ON u.id = a.user_id
	`

	baseWhere := []string{"a.deleted_at IS NULL"}
	args := []any{limit}

	whereClause, args := buildActionsWhere(f, 2, baseWhere, args)
	query := baseQuery + " WHERE " + whereClause

	var tooMany bool
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&tooMany); err != nil {
		return false, err
	}

	return tooMany, nil
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
