package domain

import "time"

type Action struct {
	DebtID string

	UserID int64

	DebtStatusID *int64

	NextContact *time.Time

	Type    string
	Comment string

	Payload []byte

	CreatedAt *time.Time
	UpdatedAt *time.Time
	DeletedAt *time.Time

	DebtNumber       *string
	CounterpartyName *string

	DebtStatusName *string

	UserFirstName   *string
	UserLastName    *string
	UserMiddleName  *string
	UserDepartments *string
	UserFullName    *string

	DebtorFirstName  *string
	DebtorLastName   *string
	DebtorMiddleName *string

	PayloadDatePromisedPayment   *string
	PayloadAmountPromisedPayment *float64
}
