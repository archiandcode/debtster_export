package domain

import "time"

type Payment struct {
	ID                           string
	DebtID                       string
	UserID                       *int64
	Amount                       float64
	AmountAfterSubtraction       float64
	AmountGovernmentDuty         float64
	AmountRepresentationExpenses float64
	AmountNotaryFees             float64
	AmountPostage                float64
	Confirmed                    bool
	PaymentDate                  *time.Time

	CreatedAt *time.Time
	UpdatedAt *time.Time
	DeletedAt *time.Time

	// additional amounts
	AmountAccountsReceivable float64
	AmountMainDebt           float64
	AmountAccrual            float64
	AmountFine               float64
}
