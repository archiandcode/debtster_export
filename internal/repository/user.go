package repository

import (
	"context"
	"database/sql"

	"debtster-export/internal/domain"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) List(ctx context.Context) ([]domain.User, error) {
	baseQuery := `
		SELECT
			u.first_name,
			u.last_name,
			u.middle_name,
			u.username,
			u.email,
			u.phone,
			ud.departments
		FROM users u
		LEFT JOIN (
			SELECT
				du.user_id,
				string_agg(d.display_name, ', ' ORDER BY d.display_name) AS departments
			FROM department_user du
			JOIN departments d ON d.id = du.department_id
			GROUP BY du.user_id
		) ud ON ud.user_id = u.id
		WHERE u.deleted_at IS NULL
	`

	rows, err := r.db.QueryContext(ctx, baseQuery)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []domain.User

	for rows.Next() {
		var u domain.User

		if err := rows.Scan(
			&u.FirstName,
			&u.LastName,
			&u.MiddleName,
			&u.Username,
			&u.Email,
			&u.Phone,
			&u.Departments,
		); err != nil {
			return nil, err
		}

		result = append(result, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
