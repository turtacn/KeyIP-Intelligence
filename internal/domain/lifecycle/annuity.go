package lifecycle

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// AnnuityStatus defines the status of an annuity payment.
type AnnuityStatus string

const (
	AnnuityStatusPending     AnnuityStatus = "pending"
	AnnuityStatusPaid        AnnuityStatus = "paid"
	AnnuityStatusOverdue     AnnuityStatus = "overdue"
	AnnuityStatusGracePeriod AnnuityStatus = "grace_period"
	AnnuityStatusAbandoned   AnnuityStatus = "abandoned"
	// Legacy status for compatibility
	AnnuityStatusUpcoming    AnnuityStatus = "upcoming"
	AnnuityStatusDue         AnnuityStatus = "due"
	AnnuityStatusWaived      AnnuityStatus = "waived"
	AnnuityStatusExpired     AnnuityStatus = "expired"
)

// Money represents a monetary value.
type Money struct {
	Amount   int64  `json:"amount"`   // In smallest unit (e.g., cents)
	Currency string `json:"currency"` // ISO 4217 code
}

// NewMoney creates a new Money instance.
func NewMoney(amount int64, currency string) Money {
	return Money{
		Amount:   amount,
		Currency: currency,
	}
}

// ToFloat64 converts Money to float64.
func (m Money) ToFloat64() float64 {
	return float64(m.Amount) / 100.0
}

// Add adds another Money value.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("cannot add different currencies: %s and %s", m.Currency, other.Currency)
	}
	return Money{
		Amount:   m.Amount + other.Amount,
		Currency: m.Currency,
	}, nil
}

// Validate checks if the Money value is valid.
func (m Money) Validate() error {
	if m.Amount < 0 {
		return fmt.Errorf("amount cannot be negative: %d", m.Amount)
	}
	if m.Currency == "" {
		return fmt.Errorf("currency cannot be empty")
	}
	return nil
}

// AnnuityRecord represents a single annuity payment record.
// Updated to include legacy fields for compatibility.
type AnnuityRecord struct {
	ID               string                 `json:"id"`
	PatentID         string                 `json:"patent_id"`
	JurisdictionCode string                 `json:"jurisdiction_code"`
	YearNumber       int                    `json:"year_number"`
	DueDate          time.Time              `json:"due_date"`
	GraceDeadline    time.Time              `json:"grace_deadline"`
	Amount           Money                  `json:"amount"`
	PaidAmount       *Money                 `json:"paid_amount"`
	PaidDate         *time.Time             `json:"paid_date"`
	Status           AnnuityStatus          `json:"status"`
	Currency         string                 `json:"currency"`
	Notes            string                 `json:"notes"`
	PaymentReference string                 `json:"payment_reference"` // Added
	AgentName        string                 `json:"agent_name"`        // Added
	AgentReference   string                 `json:"agent_reference"`   // Added
	ReminderSentAt   *time.Time             `json:"reminder_sent_at"`  // Added
	ReminderCount    int                    `json:"reminder_count"`    // Added
	Metadata         map[string]interface{} `json:"metadata"`          // Added
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// Alias for compatibility
type Annuity = AnnuityRecord

// AnnuitySchedule represents a generated schedule of annuity payments.
type AnnuitySchedule struct {
	PatentID         string           `json:"patent_id"`
	JurisdictionCode string           `json:"jurisdiction_code"`
	Records          []*AnnuityRecord `json:"records"`
	TotalEstimatedCost Money            `json:"total_estimated_cost"`
	NextDueRecord    *AnnuityRecord   `json:"next_due_record"`
	GeneratedAt      time.Time        `json:"generated_at"`
}

// AnnuityCostForecast represents a forecast of annuity costs.
type AnnuityCostForecast struct {
	PortfolioID       string           `json:"portfolio_id"`
	ForecastYears     int              `json:"forecast_years"`
	YearlyCosts       map[int]Money    `json:"yearly_costs"`
	TotalForecastCost Money            `json:"total_forecast_cost"`
	CostByJurisdiction map[string]Money `json:"cost_by_jurisdiction"`
	CostByPatent      map[string]Money `json:"cost_by_patent"`
	GeneratedAt       time.Time        `json:"generated_at"`
}

// AnnuityService defines the interface for annuity management.
type AnnuityService interface {
	GenerateSchedule(ctx context.Context, patentID, jurisdictionCode string, filingDate time.Time, maxYears int) (*AnnuitySchedule, error)
	CalculateAnnuityFee(ctx context.Context, jurisdictionCode string, yearNumber int) (Money, error)
	MarkAsPaid(ctx context.Context, recordID string, paidAmount Money, paidDate time.Time) error
	MarkAsAbandoned(ctx context.Context, recordID string, reason string) error
	CheckOverdue(ctx context.Context, asOfDate time.Time) ([]*AnnuityRecord, error)
	ForecastCosts(ctx context.Context, portfolioID string, years int) (*AnnuityCostForecast, error)
	GetUpcomingPayments(ctx context.Context, portfolioID string, withinDays int) ([]*AnnuityRecord, error)
}

type annuityServiceImpl struct {
	annuityRepo   AnnuityRepository
	portfolioRepo portfolio.PortfolioRepository
	jurisdictionReg JurisdictionRegistry
}

// NewAnnuityService creates a new AnnuityService.
func NewAnnuityService(annuityRepo AnnuityRepository, portfolioRepo portfolio.PortfolioRepository, reg JurisdictionRegistry) AnnuityService {
	return &annuityServiceImpl{
		annuityRepo:   annuityRepo,
		portfolioRepo: portfolioRepo,
		jurisdictionReg: reg,
	}
}

func (s *annuityServiceImpl) GenerateSchedule(ctx context.Context, patentID, jurisdictionCode string, filingDate time.Time, maxYears int) (*AnnuitySchedule, error) {
	j, err := s.jurisdictionReg.Get(jurisdictionCode)
	if err != nil {
		return nil, err
	}
	rules, err := s.jurisdictionReg.GetAnnuityRules(jurisdictionCode)
	if err != nil {
		return nil, err
	}

	records := []*AnnuityRecord{}
	totalCost := NewMoney(0, j.Currency)

	now := time.Now().UTC()

	if rules.IsAnnual {
		for year := rules.StartYear; year <= maxYears; year++ {
			fee, err := s.CalculateAnnuityFee(ctx, jurisdictionCode, year)
			if err != nil {
				return nil, err
			}

			dueDate := CalculateAnnuityDueDate(filingDate, year)

			rec := &AnnuityRecord{
				ID:               uuid.New().String(),
				PatentID:         patentID,
				JurisdictionCode: jurisdictionCode,
				YearNumber:       year,
				DueDate:          dueDate,
				GraceDeadline:    CalculateGraceDeadline(dueDate, rules.GracePeriodMonths),
				Amount:           fee,
				Status:           AnnuityStatusPending,
				Currency:         j.Currency,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			records = append(records, rec)

			newTotal, _ := totalCost.Add(fee)
			totalCost = newTotal
		}
	} else {
		for _, milestone := range rules.PaymentSchedule {
			year := int(math.Floor(milestone.YearMark))

			dueDate := filingDate.AddDate(0, int(milestone.YearMark*12), 0)

			fee, err := s.CalculateAnnuityFee(ctx, jurisdictionCode, year)
			if err != nil {
				return nil, err
			}

			rec := &AnnuityRecord{
				ID:               uuid.New().String(),
				PatentID:         patentID,
				JurisdictionCode: jurisdictionCode,
				YearNumber:       year,
				DueDate:          dueDate,
				GraceDeadline:    CalculateGraceDeadline(dueDate, rules.GracePeriodMonths),
				Amount:           fee,
				Status:           AnnuityStatusPending,
				Currency:         j.Currency,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			records = append(records, rec)

			newTotal, _ := totalCost.Add(fee)
			totalCost = newTotal
		}
	}

	return &AnnuitySchedule{
		PatentID:           patentID,
		JurisdictionCode:   jurisdictionCode,
		Records:            records,
		TotalEstimatedCost: totalCost,
		GeneratedAt:        now,
	}, nil
}

func (s *annuityServiceImpl) CalculateAnnuityFee(ctx context.Context, jurisdictionCode string, yearNumber int) (Money, error) {
	switch jurisdictionCode {
	case "CN":
		amount := int64(0)
		if yearNumber >= 3 && yearNumber <= 6 {
			amount = 90000
		} else if yearNumber >= 7 && yearNumber <= 9 {
			amount = 120000
		} else if yearNumber >= 10 && yearNumber <= 12 {
			amount = 200000
		} else if yearNumber >= 13 && yearNumber <= 15 {
			amount = 400000
		} else if yearNumber >= 16 && yearNumber <= 20 {
			amount = 600000
		}
		return NewMoney(amount, "CNY"), nil
	case "US":
		amount := int64(0)
		if yearNumber == 3 {
			amount = 160000
		} else if yearNumber == 7 {
			amount = 360000
		} else if yearNumber == 11 {
			amount = 740000
		}
		return NewMoney(amount, "USD"), nil
	case "EP":
		if yearNumber < 3 {
			return NewMoney(0, "EUR"), nil
		}
		base := int64(47000)
		inc := int64(5000) * int64(yearNumber-3)
		return NewMoney(base+inc, "EUR"), nil
	case "JP":
		if yearNumber <= 3 {
			return NewMoney(1160000, "JPY"), nil
		}
		return NewMoney(2000000, "JPY"), nil
	default:
		return NewMoney(10000, "USD"), nil
	}
}

func (s *annuityServiceImpl) MarkAsPaid(ctx context.Context, recordID string, paidAmount Money, paidDate time.Time) error {
	rec, err := s.annuityRepo.GetAnnuityByID(ctx, recordID)
	if err != nil {
		return err
	}
	if rec == nil {
		return apperrors.NewNotFound("annuity record not found: %s", recordID)
	}

	if rec.Status == AnnuityStatusPaid {
		return apperrors.NewValidation("annuity already paid")
	}
	if rec.Status == AnnuityStatusAbandoned {
		return apperrors.NewValidation("cannot pay abandoned annuity")
	}

	rec.Status = AnnuityStatusPaid
	rec.PaidAmount = &paidAmount
	rec.PaidDate = &paidDate
	rec.UpdatedAt = time.Now().UTC()

	return s.annuityRepo.SaveAnnuity(ctx, rec)
}

func (s *annuityServiceImpl) MarkAsAbandoned(ctx context.Context, recordID string, reason string) error {
	rec, err := s.annuityRepo.GetAnnuityByID(ctx, recordID)
	if err != nil {
		return err
	}
	if rec == nil {
		return apperrors.NewNotFound("annuity record not found: %s", recordID)
	}

	rec.Status = AnnuityStatusAbandoned
	rec.Notes = reason
	rec.UpdatedAt = time.Now().UTC()

	return s.annuityRepo.SaveAnnuity(ctx, rec)
}

func (s *annuityServiceImpl) CheckOverdue(ctx context.Context, asOfDate time.Time) ([]*AnnuityRecord, error) {
	pending, err := s.annuityRepo.GetPendingAnnuities(ctx, asOfDate)
	if err != nil {
		return nil, err
	}

	var updated []*AnnuityRecord
	for _, rec := range pending {
		if rec.GraceDeadline.After(asOfDate) {
			if rec.Status != AnnuityStatusGracePeriod {
				rec.Status = AnnuityStatusGracePeriod
				rec.UpdatedAt = time.Now().UTC()
				if err := s.annuityRepo.SaveAnnuity(ctx, rec); err != nil {
					return nil, err
				}
				updated = append(updated, rec)
			}
		} else {
			if rec.Status != AnnuityStatusOverdue {
				rec.Status = AnnuityStatusOverdue
				rec.UpdatedAt = time.Now().UTC()
				if err := s.annuityRepo.SaveAnnuity(ctx, rec); err != nil {
					return nil, err
				}
				updated = append(updated, rec)
			}
		}
	}
	return updated, nil
}

func (s *annuityServiceImpl) ForecastCosts(ctx context.Context, portfolioID string, years int) (*AnnuityCostForecast, error) {
	p, err := s.portfolioRepo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperrors.NewNotFound("portfolio not found: %s", portfolioID)
	}

	forecast := &AnnuityCostForecast{
		PortfolioID:       portfolioID,
		ForecastYears:     years,
		YearlyCosts:       make(map[int]Money),
		TotalForecastCost: NewMoney(0, "USD"),
		CostByJurisdiction: make(map[string]Money),
		CostByPatent:      make(map[string]Money),
		GeneratedAt:       time.Now().UTC(),
	}

	exchangeRates := map[string]float64{
		"USD": 1.0,
		"CNY": 0.14,
		"EUR": 1.1,
		"JPY": 0.0067,
		"KRW": 0.00075,
		"GBP": 1.25,
	}

	totalUSD := 0.0

	for _, pid := range p.PatentIDs {
		records, err := s.annuityRepo.GetAnnuitiesByPatentID(ctx, pid)
		if err != nil {
			return nil, err
		}

		for _, rec := range records {
			if rec.Status == AnnuityStatusPaid || rec.Status == AnnuityStatusAbandoned {
				continue
			}
			if rec.DueDate.Year() > time.Now().Year() + years {
				continue
			}

			currentPatentCost := forecast.CostByPatent[pid]
			if currentPatentCost.Currency == "" {
				currentPatentCost = NewMoney(0, rec.Currency)
			}
			newPatentCost, _ := currentPatentCost.Add(rec.Amount)
			forecast.CostByPatent[pid] = newPatentCost

			currentJurCost := forecast.CostByJurisdiction[rec.JurisdictionCode]
			if currentJurCost.Currency == "" {
				currentJurCost = NewMoney(0, rec.Currency)
			}
			newJurCost, _ := currentJurCost.Add(rec.Amount)
			forecast.CostByJurisdiction[rec.JurisdictionCode] = newJurCost

			rate := exchangeRates[rec.Currency]
			if rate == 0 { rate = 1.0 }
			valUSD := rec.Amount.ToFloat64() * rate
			totalUSD += valUSD
		}
	}

	forecast.TotalForecastCost = NewMoney(int64(totalUSD * 100), "USD")
	return forecast, nil
}

func (s *annuityServiceImpl) GetUpcomingPayments(ctx context.Context, portfolioID string, withinDays int) ([]*AnnuityRecord, error) {
	p, err := s.portfolioRepo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, apperrors.NewNotFound("portfolio not found: %s", portfolioID)
	}

	limitDate := time.Now().UTC().AddDate(0, 0, withinDays)
	var upcoming []*AnnuityRecord

	for _, pid := range p.PatentIDs {
		records, err := s.annuityRepo.GetAnnuitiesByPatentID(ctx, pid)
		if err != nil {
			return nil, err
		}
		for _, rec := range records {
			if rec.Status == AnnuityStatusPending {
				if rec.DueDate.Before(limitDate) && rec.DueDate.After(time.Now().UTC()) {
					upcoming = append(upcoming, rec)
				}
			}
		}
	}
	return upcoming, nil
}

//Personal.AI order the ending
