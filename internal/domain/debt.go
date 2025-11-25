package domain

import "time"

type Debt struct {
	Number string

	StartDate *time.Time
	EndDate   *time.Time
	Filial    *string

	ProductName    *string
	AmountCurrency *string

	AmountActualDebt        float64
	AmountPurchasedLoan     float64
	InitAmountActualDebt    float64
	AmountCredit            float64
	AmountMainDebt          *float64
	AmountFine              float64
	AmountAccrual           float64
	AmountGovernmentDuty    float64
	AmountRepresentationExp float64
	AmountNotaryFees        float64
	AmountPostage           float64

	TransferDecision           *string
	PresenceSolidarity         bool
	GovernmentDutyPaid         bool
	GovernmentDutyRefund       bool
	RepresentationExpensesPaid bool

	LateDueDate *time.Time
	NextContact *time.Time
	LastContact *time.Time

	AdditionalData []byte

	RegistryNumber *string
	RegistryDate   *time.Time

	UserUsername    *string
	UserDepartments *string

	StatusName *string

	DebtorLastName   *string
	DebtorFirstName  *string
	DebtorMiddleName *string
	DebtorIIN        *string

	CounterpartyName *string
}
