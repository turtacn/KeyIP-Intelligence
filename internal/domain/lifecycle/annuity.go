package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// AnnuityStatus defines the status of an annuity payment.
type AnnuityStatus string

const (
	AnnuityStatusPending     AnnuityStatus = "Pending"
	AnnuityStatusPaid        AnnuityStatus = "Paid"
	AnnuityStatusOverdue     AnnuityStatus = "Overdue"
	AnnuityStatusGracePeriod AnnuityStatus = "GracePeriod"
	AnnuityStatusAbandoned   AnnuityStatus = "Abandoned"
)

// Money is a value object representing an amount of money in a specific currency.
type Money struct {
	Amount   int64  `json:"amount"`   // Amount in smallest currency unit (e.g., cents)
	Currency string `json:"currency"` // ISO 4217 currency code
}

// NewMoney creates a new Money value object.
func NewMoney(amount int64, currency string) Money {
	return Money{
		Amount:   amount,
		Currency: currency,
	}
}

// ToFloat64 converts the money amount to a float64 (major currency unit).
func (m Money) ToFloat64() float64 {
	return float64(m.Amount) / 100.0
}

// Add adds another Money object to this one, provided they have the same currency.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, errors.InvalidParam("cannot add money with different currencies")
	}
	return Money{
		Amount:   m.Amount + other.Amount,
		Currency: m.Currency,
	}, nil
}

// Validate ensures the money object is valid.
func (m Money) Validate() error {
	if m.Amount < 0 {
		return errors.InvalidParam("money amount cannot be negative")
	}
	if m.Currency == "" {
		return errors.InvalidParam("currency code cannot be empty")
	}
	return nil
}

// AnnuityRecord represents a single annuity payment record.
type AnnuityRecord struct {
	ID               string        `json:"id"`
	PatentID         string        `json:"patent_id"`
	JurisdictionCode string        `json:"jurisdiction_code"`
	YearNumber       int           `json:"year_number"`
	DueDate          time.Time     `json:"due_date"`
	GraceDeadline    time.Time     `json:"grace_deadline"`
	Amount           Money         `json:"amount"`
	PaidAmount       *Money        `json:"paid_amount,omitempty"`
	PaidDate         *time.Time    `json:"paid_date,omitempty"`
	Status           AnnuityStatus `json:"status"`
	Currency         string        `json:"currency"`
	Notes            string        `json:"notes"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// AnnuitySchedule represents a collection of annuity records for a patent.
type AnnuitySchedule struct {
	PatentID           string           `json:"patent_id"`
	JurisdictionCode   string           `json:"jurisdiction_code"`
	Records            []*AnnuityRecord `json:"records"`
	TotalEstimatedCost Money            `json:"total_estimated_cost"`
	NextDueRecord      *AnnuityRecord   `json:"next_due_record"`
	GeneratedAt        time.Time        `json:"generated_at"`
}

// AnnuityCostForecast represents a forecast of annuity costs for a portfolio.
type AnnuityCostForecast struct {
	PortfolioID        string           `json:"portfolio_id"`
	ForecastYears      int              `json:"forecast_years"`
	YearlyCosts        map[int]Money    `json:"yearly_costs"`
	TotalForecastCost  Money            `json:"total_forecast_cost"`
	CostByJurisdiction map[string]Money `json:"cost_by_jurisdiction"`
	CostByPatent       map[string]Money `json:"cost_by_patent"`
	GeneratedAt        time.Time        `json:"generated_at"`
}

// AnnuityService defines the domain service for managing annuities.
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
}

// NewAnnuityService creates a new AnnuityService.
func NewAnnuityService(annuityRepo AnnuityRepository, portfolioRepo portfolio.PortfolioRepository) AnnuityService {
	return &annuityServiceImpl{
		annuityRepo:   annuityRepo,
		portfolioRepo: portfolioRepo,
	}
}

func (s *annuityServiceImpl) GenerateSchedule(ctx context.Context, patentID, jurisdictionCode string, filingDate time.Time, maxYears int) (*AnnuitySchedule, error) {
	var records []*AnnuityRecord
	var totalAmount int64
	var currency string

	switch jurisdictionCode {
	case "CN":
		currency = "CNY"
		for y := 3; y <= 20 && y <= maxYears; y++ {
			fee, _ := s.CalculateAnnuityFee(ctx, "CN", y)
			dueDate := filingDate.AddDate(y, 0, 0)
			graceDeadline := dueDate.AddDate(0, 6, 0)
			record := &AnnuityRecord{
				ID:               uuid.New().String(),
				PatentID:         patentID,
				JurisdictionCode: "CN",
				YearNumber:       y,
				DueDate:          dueDate,
				GraceDeadline:    graceDeadline,
				Amount:           fee,
				Status:           AnnuityStatusPending,
				Currency:         currency,
				CreatedAt:        time.Now().UTC(),
				UpdatedAt:        time.Now().UTC(),
			}
			records = append(records, record)
			totalAmount += fee.Amount
		}
	case "US":
		currency = "USD"
		milestones := []struct {
			year float64
			val  int
		}{
			{3.5, 4},
			{7.5, 8},
			{11.5, 12},
		}
		for _, m := range milestones {
			if int(m.year) > maxYears {
				continue
			}
			fee, _ := s.CalculateAnnuityFee(ctx, "US", m.val)
			months := int(m.year * 12)
			dueDate := filingDate.AddDate(0, months, 0)
			graceDeadline := dueDate.AddDate(0, 6, 0)
			record := &AnnuityRecord{
				ID:               uuid.New().String(),
				PatentID:         patentID,
				JurisdictionCode: "US",
				YearNumber:       m.val,
				DueDate:          dueDate,
				GraceDeadline:    graceDeadline,
				Amount:           fee,
				Status:           AnnuityStatusPending,
				Currency:         currency,
				CreatedAt:        time.Now().UTC(),
				UpdatedAt:        time.Now().UTC(),
			}
			records = append(records, record)
			totalAmount += fee.Amount
		}
	case "EP":
		currency = "EUR"
		for y := 3; y <= 20 && y <= maxYears; y++ {
			fee, _ := s.CalculateAnnuityFee(ctx, "EP", y)
			dueDate := filingDate.AddDate(y, 0, 0)
			graceDeadline := dueDate.AddDate(0, 6, 0)
			record := &AnnuityRecord{
				ID:               uuid.New().String(),
				PatentID:         patentID,
				JurisdictionCode: "EP",
				YearNumber:       y,
				DueDate:          dueDate,
				GraceDeadline:    graceDeadline,
				Amount:           fee,
				Status:           AnnuityStatusPending,
				Currency:         currency,
				CreatedAt:        time.Now().UTC(),
				UpdatedAt:        time.Now().UTC(),
			}
			records = append(records, record)
			totalAmount += fee.Amount
		}
	case "JP":
		currency = "JPY"
		for y := 1; y <= 20 && y <= maxYears; y++ {
			fee, _ := s.CalculateAnnuityFee(ctx, "JP", y)
			dueDate := filingDate.AddDate(y, 0, 0)
			graceDeadline := dueDate.AddDate(0, 6, 0)
			record := &AnnuityRecord{
				ID:               uuid.New().String(),
				PatentID:         patentID,
				JurisdictionCode: "JP",
				YearNumber:       y,
				DueDate:          dueDate,
				GraceDeadline:    graceDeadline,
				Amount:           fee,
				Status:           AnnuityStatusPending,
				Currency:         currency,
				CreatedAt:        time.Now().UTC(),
				UpdatedAt:        time.Now().UTC(),
			}
			records = append(records, record)
			totalAmount += fee.Amount
		}
	default:
		return nil, errors.InvalidParam(fmt.Sprintf("unsupported jurisdiction: %s", jurisdictionCode))
	}

	schedule := &AnnuitySchedule{
		PatentID:           patentID,
		JurisdictionCode:   jurisdictionCode,
		Records:            records,
		TotalEstimatedCost: NewMoney(totalAmount, currency),
		GeneratedAt:        time.Now().UTC(),
	}

	if len(records) > 0 {
		schedule.NextDueRecord = records[0]
	}

	return schedule, nil
}

func (s *annuityServiceImpl) CalculateAnnuityFee(ctx context.Context, jurisdictionCode string, yearNumber int) (Money, error) {
	switch jurisdictionCode {
	case "CN":
		var amount int64
		switch {
		case yearNumber >= 3 && yearNumber <= 6:
			amount = 90000
		case yearNumber >= 7 && yearNumber <= 9:
			amount = 120000
		case yearNumber >= 10 && yearNumber <= 12:
			amount = 200000
		case yearNumber >= 13 && yearNumber <= 15:
			amount = 400000
		case yearNumber >= 16 && yearNumber <= 20:
			amount = 600000
		default:
			return Money{}, errors.InvalidParam("invalid year for CN annuity")
		}
		return NewMoney(amount, "CNY"), nil
	case "US":
		var amount int64
		switch yearNumber {
		case 4: // 3.5 years
			amount = 160000
		case 8: // 7.5 years
			amount = 360000
		case 12: // 11.5 years
			amount = 740000
		default:
			return Money{}, errors.InvalidParam("invalid year for US maintenance fee")
		}
		return NewMoney(amount, "USD"), nil
	case "EP":
		if yearNumber < 3 || yearNumber > 20 {
			return Money{}, errors.InvalidParam("invalid year for EP annuity")
		}
		amount := int64(47000 + (yearNumber-3)*7500) // approx 50-100 increase
		return NewMoney(amount, "EUR"), nil
	case "JP":
		if yearNumber < 1 || yearNumber > 20 {
			return Money{}, errors.InvalidParam("invalid year for JP annuity")
		}
		// JP: 6600 + claims * 500 (simplified to 10000 + year * 2000)
		amount := int64(1000000 + int64(yearNumber)*200000)
		return NewMoney(amount, "JPY"), nil
	}
	return Money{}, errors.InvalidParam(fmt.Sprintf("unsupported jurisdiction: %s", jurisdictionCode))
}

func (s *annuityServiceImpl) MarkAsPaid(ctx context.Context, recordID string, paidAmount Money, paidDate time.Time) error {
	record, err := s.annuityRepo.FindByID(ctx, recordID)
	if err != nil {
		return err
	}
	if record.Status == AnnuityStatusPaid {
		return errors.InvalidState("annuity already paid")
	}
	if record.Status == AnnuityStatusAbandoned {
		return errors.InvalidState("cannot pay an abandoned annuity")
	}

	record.Status = AnnuityStatusPaid
	record.PaidAmount = &paidAmount
	record.PaidDate = &paidDate
	record.UpdatedAt = time.Now().UTC()

	return s.annuityRepo.Save(ctx, record)
}

func (s *annuityServiceImpl) MarkAsAbandoned(ctx context.Context, recordID string, reason string) error {
	record, err := s.annuityRepo.FindByID(ctx, recordID)
	if err != nil {
		return err
	}
	record.Status = AnnuityStatusAbandoned
	record.Notes = reason
	record.UpdatedAt = time.Now().UTC()
	return s.annuityRepo.Save(ctx, record)
}

func (s *annuityServiceImpl) CheckOverdue(ctx context.Context, asOfDate time.Time) ([]*AnnuityRecord, error) {
	records, err := s.annuityRepo.FindPending(ctx, asOfDate)
	if err != nil {
		return nil, err
	}

	var changed []*AnnuityRecord
	for _, r := range records {
		originalStatus := r.Status
		if asOfDate.After(r.GraceDeadline) {
			r.Status = AnnuityStatusOverdue
		} else if asOfDate.After(r.DueDate) {
			r.Status = AnnuityStatusGracePeriod
		}

		if r.Status != originalStatus {
			r.UpdatedAt = time.Now().UTC()
			if err := s.annuityRepo.Save(ctx, r); err != nil {
				return nil, err
			}
			changed = append(changed, r)
		}
	}
	return changed, nil
}

func (s *annuityServiceImpl) ForecastCosts(ctx context.Context, portfolioID string, years int) (*AnnuityCostForecast, error) {
	p, err := s.portfolioRepo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	forecast := &AnnuityCostForecast{
		PortfolioID:        portfolioID,
		ForecastYears:      years,
		YearlyCosts:        make(map[int]Money),
		CostByJurisdiction: make(map[string]Money),
		CostByPatent:       make(map[string]Money),
		GeneratedAt:        time.Now().UTC(),
	}

	now := time.Now().UTC()
	end := now.AddDate(years, 0, 0)

	var totalAmount int64
	var commonCurrency string

	for _, patentID := range p.PatentIDs {
		records, err := s.annuityRepo.FindByPatentID(ctx, patentID)
		if err != nil {
			continue
		}

		var patentTotal int64
		var patentCurrency string

		for _, r := range records {
			if r.Status == AnnuityStatusPending || r.Status == AnnuityStatusGracePeriod {
				if r.DueDate.After(now) && r.DueDate.Before(end) {
					year := r.DueDate.Year()

					// Update yearly costs
					yc := forecast.YearlyCosts[year]
					if yc.Currency == "" {
						yc.Currency = r.Currency
					}
					if yc.Currency == r.Currency {
						yc.Amount += r.Amount.Amount
						forecast.YearlyCosts[year] = yc
					}

					// Update jurisdiction costs
					jc := forecast.CostByJurisdiction[r.JurisdictionCode]
					if jc.Currency == "" {
						jc.Currency = r.Currency
					}
					if jc.Currency == r.Currency {
						jc.Amount += r.Amount.Amount
						forecast.CostByJurisdiction[r.JurisdictionCode] = jc
					}

					patentTotal += r.Amount.Amount
					patentCurrency = r.Currency

					if commonCurrency == "" {
						commonCurrency = r.Currency
					}
					if commonCurrency == r.Currency {
						totalAmount += r.Amount.Amount
					}
				}
			}
		}
		forecast.CostByPatent[patentID] = NewMoney(patentTotal, patentCurrency)
	}

	forecast.TotalForecastCost = NewMoney(totalAmount, commonCurrency)
	return forecast, nil
}

func (s *annuityServiceImpl) GetUpcomingPayments(ctx context.Context, portfolioID string, withinDays int) ([]*AnnuityRecord, error) {
	p, err := s.portfolioRepo.FindByID(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, withinDays)

	var upcoming []*AnnuityRecord
	for _, patentID := range p.PatentIDs {
		records, err := s.annuityRepo.FindByPatentID(ctx, patentID)
		if err != nil {
			continue
		}
		for _, r := range records {
			if (r.Status == AnnuityStatusPending || r.Status == AnnuityStatusGracePeriod) &&
				r.DueDate.After(now) && r.DueDate.Before(cutoff) {
				upcoming = append(upcoming, r)
			}
		}
	}
	return upcoming, nil
}

//Personal.AI order the ending
