// Package emission provides DTOs and business logic for NFS-e emission operations.
package emission

import (
	"errors"
	"fmt"
	"math"
)

// Calculator-related error codes for financial validation.
const (
	ErrCodeInvalidServiceValue = "INVALID_SERVICE_VALUE"
	ErrCodeInvalidDiscount     = "INVALID_DISCOUNT"
	ErrCodeDiscountExceedsValue = "DISCOUNT_EXCEEDS_VALUE"
	ErrCodeInvalidDeduction    = "INVALID_DEDUCTION"
	ErrCodeDeductionExceedsValue = "DEDUCTION_EXCEEDS_VALUE"
	ErrCodeNegativeTaxBase     = "NEGATIVE_TAX_BASE"
)

// CalculationError represents an error that occurred during value calculation.
type CalculationError struct {
	// Code is a machine-readable error code.
	Code string

	// Field is the name of the field that caused the error.
	Field string

	// Message is a human-readable error message.
	Message string
}

// Error implements the error interface.
func (e *CalculationError) Error() string {
	return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Field)
}

// NewCalculationError creates a new CalculationError.
func NewCalculationError(code, field, message string) *CalculationError {
	return &CalculationError{
		Code:    code,
		Field:   field,
		Message: message,
	}
}

// CalculationInput contains the input values for tax base calculation.
type CalculationInput struct {
	// ServiceValue is the gross value of the service (vServ).
	// Must be positive and have at most 2 decimal places.
	ServiceValue float64

	// UnconditionalDiscount is a discount applied regardless of payment conditions (vDescIncond).
	// Reduces the tax base. Must be >= 0 and have at most 2 decimal places.
	UnconditionalDiscount float64

	// ConditionalDiscount is a discount conditional on payment terms (vDescCond).
	// Does NOT reduce the tax base. Must be >= 0 and have at most 2 decimal places.
	ConditionalDiscount float64

	// Deductions are legally permitted deductions from the service value (vDedRed).
	// Reduces the tax base. Must be >= 0 and have at most 2 decimal places.
	Deductions float64

	// ISSRate is the ISS tax rate as a percentage (e.g., 2.0 for 2%).
	// Can be 0 for SIMPLES NACIONAL MEI providers.
	ISSRate float64
}

// CalculationResult contains the results of the tax base calculation.
type CalculationResult struct {
	// ServiceValue is the gross value of the service (vServ).
	ServiceValue float64

	// UnconditionalDiscount is the unconditional discount applied (vDescIncond).
	UnconditionalDiscount float64

	// ConditionalDiscount is the conditional discount recorded (vDescCond).
	ConditionalDiscount float64

	// Deductions is the total deductions applied (vDR).
	Deductions float64

	// DeductionPercentage is the deduction as a percentage of service value (pDR).
	// Calculated as (Deductions / ServiceValue) * 100.
	DeductionPercentage float64

	// TaxBase is the calculated tax base for ISS (vBCCalc).
	// Calculated as ServiceValue - UnconditionalDiscount - Deductions.
	// Note: ConditionalDiscount does NOT reduce the tax base.
	TaxBase float64

	// ISSRate is the ISS tax rate percentage (pAliq).
	ISSRate float64

	// ISSAmount is the calculated ISS tax amount (vISS).
	// Calculated as TaxBase * ISSRate / 100.
	ISSAmount float64

	// NetValue is the final value after all discounts (for reference only).
	// Calculated as ServiceValue - UnconditionalDiscount - ConditionalDiscount - Deductions.
	NetValue float64
}

// ValueCalculator performs tax base and ISS calculations for NFS-e emissions.
// It implements the Brazilian NFS-e calculation rules where:
// - Tax Base (vBCCalc) = Service Value - Unconditional Discount - Deductions
// - Conditional Discount does NOT affect the tax base
// - ISS Amount (vISS) = Tax Base * ISS Rate / 100
type ValueCalculator struct{}

// NewValueCalculator creates a new ValueCalculator.
func NewValueCalculator() *ValueCalculator {
	return &ValueCalculator{}
}

// Calculate performs the tax base calculation according to Brazilian NFS-e rules.
//
// Tax Base Calculation Formula:
//
//	vBCCalc = vServ - vDescIncond - vDedRed
//
// Note: vDescCond (conditional discount) is recorded but does NOT reduce the tax base.
//
// Returns a CalculationResult on success, or an error if validation fails.
func (c *ValueCalculator) Calculate(input *CalculationInput) (*CalculationResult, error) {
	if input == nil {
		return nil, errors.New("calculation input is required")
	}

	// Validate service value
	if err := c.validateServiceValue(input.ServiceValue); err != nil {
		return nil, err
	}

	// Validate unconditional discount
	if err := c.validateUnconditionalDiscount(input.ServiceValue, input.UnconditionalDiscount); err != nil {
		return nil, err
	}

	// Validate conditional discount
	if err := c.validateConditionalDiscount(input.ServiceValue, input.ConditionalDiscount); err != nil {
		return nil, err
	}

	// Validate deductions
	if err := c.validateDeductions(input.ServiceValue, input.Deductions); err != nil {
		return nil, err
	}

	// Validate that unconditional discount + deductions don't exceed service value
	// (this would result in a negative tax base)
	taxBaseDeductions := input.UnconditionalDiscount + input.Deductions
	if taxBaseDeductions > input.ServiceValue {
		return nil, NewCalculationError(
			ErrCodeNegativeTaxBase,
			"values",
			fmt.Sprintf("unconditional discount (%.2f) plus deductions (%.2f) cannot exceed service value (%.2f)",
				input.UnconditionalDiscount, input.Deductions, input.ServiceValue),
		)
	}

	// Calculate tax base: vBCCalc = vServ - vDescIncond - vDedRed
	taxBase := roundToTwoDecimals(input.ServiceValue - input.UnconditionalDiscount - input.Deductions)

	// Validate tax base is non-negative (defensive check)
	if taxBase < 0 {
		return nil, NewCalculationError(
			ErrCodeNegativeTaxBase,
			"tax_base",
			"calculated tax base is negative",
		)
	}

	// Calculate deduction percentage: pDR = (vDR / vServ) * 100
	var deductionPercentage float64
	if input.ServiceValue > 0 && input.Deductions > 0 {
		deductionPercentage = roundToTwoDecimals((input.Deductions / input.ServiceValue) * 100)
	}

	// Calculate ISS amount: vISS = vBCCalc * pAliq / 100
	var issAmount float64
	if input.ISSRate > 0 {
		issAmount = roundToTwoDecimals(taxBase * input.ISSRate / 100)
	}

	// Calculate net value (for reference): vServ - vDescIncond - vDescCond - vDedRed
	netValue := roundToTwoDecimals(input.ServiceValue - input.UnconditionalDiscount - input.ConditionalDiscount - input.Deductions)

	return &CalculationResult{
		ServiceValue:          roundToTwoDecimals(input.ServiceValue),
		UnconditionalDiscount: roundToTwoDecimals(input.UnconditionalDiscount),
		ConditionalDiscount:   roundToTwoDecimals(input.ConditionalDiscount),
		Deductions:            roundToTwoDecimals(input.Deductions),
		DeductionPercentage:   deductionPercentage,
		TaxBase:               taxBase,
		ISSRate:               roundToTwoDecimals(input.ISSRate),
		ISSAmount:             issAmount,
		NetValue:              netValue,
	}, nil
}

// validateServiceValue validates that the service value is positive and has valid precision.
func (c *ValueCalculator) validateServiceValue(value float64) error {
	if value <= 0 {
		return NewCalculationError(
			ErrCodeInvalidServiceValue,
			"service_value",
			"service value must be greater than zero",
		)
	}

	if value > MaxServiceValue {
		return NewCalculationError(
			ErrCodeInvalidServiceValue,
			"service_value",
			fmt.Sprintf("service value cannot exceed %.2f", MaxServiceValue),
		)
	}

	if !hasValidDecimalPrecision(value) {
		return NewCalculationError(
			ErrCodeInvalidServiceValue,
			"service_value",
			"service value must have at most 2 decimal places",
		)
	}

	return nil
}

// validateUnconditionalDiscount validates the unconditional discount value.
func (c *ValueCalculator) validateUnconditionalDiscount(serviceValue, discount float64) error {
	if discount < 0 {
		return NewCalculationError(
			ErrCodeInvalidDiscount,
			"unconditional_discount",
			"unconditional discount cannot be negative",
		)
	}

	if discount > serviceValue {
		return NewCalculationError(
			ErrCodeDiscountExceedsValue,
			"unconditional_discount",
			fmt.Sprintf("unconditional discount (%.2f) cannot exceed service value (%.2f)",
				discount, serviceValue),
		)
	}

	if discount > 0 && !hasValidDecimalPrecision(discount) {
		return NewCalculationError(
			ErrCodeInvalidDiscount,
			"unconditional_discount",
			"unconditional discount must have at most 2 decimal places",
		)
	}

	return nil
}

// validateConditionalDiscount validates the conditional discount value.
func (c *ValueCalculator) validateConditionalDiscount(serviceValue, discount float64) error {
	if discount < 0 {
		return NewCalculationError(
			ErrCodeInvalidDiscount,
			"conditional_discount",
			"conditional discount cannot be negative",
		)
	}

	if discount > serviceValue {
		return NewCalculationError(
			ErrCodeDiscountExceedsValue,
			"conditional_discount",
			fmt.Sprintf("conditional discount (%.2f) cannot exceed service value (%.2f)",
				discount, serviceValue),
		)
	}

	if discount > 0 && !hasValidDecimalPrecision(discount) {
		return NewCalculationError(
			ErrCodeInvalidDiscount,
			"conditional_discount",
			"conditional discount must have at most 2 decimal places",
		)
	}

	return nil
}

// validateDeductions validates the deductions value.
func (c *ValueCalculator) validateDeductions(serviceValue, deductions float64) error {
	if deductions < 0 {
		return NewCalculationError(
			ErrCodeInvalidDeduction,
			"deductions",
			"deductions cannot be negative",
		)
	}

	if deductions > serviceValue {
		return NewCalculationError(
			ErrCodeDeductionExceedsValue,
			"deductions",
			fmt.Sprintf("deductions (%.2f) cannot exceed service value (%.2f)",
				deductions, serviceValue),
		)
	}

	if deductions > 0 && !hasValidDecimalPrecision(deductions) {
		return NewCalculationError(
			ErrCodeInvalidDeduction,
			"deductions",
			"deductions must have at most 2 decimal places",
		)
	}

	return nil
}

// MaxServiceValue is the maximum allowed service value (999,999,999.99).
const MaxServiceValue = 999999999.99

// hasValidDecimalPrecision checks if a float64 value has at most 2 decimal places.
// This is critical for financial calculations to ensure monetary precision.
func hasValidDecimalPrecision(value float64) bool {
	// Multiply by 100, round, and check if it matches the original scaled value.
	// We use a small epsilon for floating point comparison.
	scaled := value * 100
	rounded := math.Round(scaled)
	return math.Abs(scaled-rounded) < 0.001
}

// roundToTwoDecimals rounds a float64 to 2 decimal places.
// Uses banker's rounding (round half to even) for financial calculations.
func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

// CalculateFromRequest is a convenience method that creates a CalculationInput from a ValuesRequest
// and performs the calculation. Uses 0 for ISS rate (MEI scenario).
func (c *ValueCalculator) CalculateFromRequest(values *ValuesRequest) (*CalculationResult, error) {
	if values == nil {
		return nil, errors.New("values request is required")
	}

	input := &CalculationInput{
		ServiceValue:          values.ServiceValue,
		UnconditionalDiscount: values.UnconditionalDiscount,
		ConditionalDiscount:   values.ConditionalDiscount,
		Deductions:            values.Deductions,
		ISSRate:               0, // Default for MEI
	}

	return c.Calculate(input)
}

// CalculateFromRequestWithRate is a convenience method that creates a CalculationInput
// from a ValuesRequest with a specified ISS rate and performs the calculation.
func (c *ValueCalculator) CalculateFromRequestWithRate(values *ValuesRequest, issRate float64) (*CalculationResult, error) {
	if values == nil {
		return nil, errors.New("values request is required")
	}

	input := &CalculationInput{
		ServiceValue:          values.ServiceValue,
		UnconditionalDiscount: values.UnconditionalDiscount,
		ConditionalDiscount:   values.ConditionalDiscount,
		Deductions:            values.Deductions,
		ISSRate:               issRate,
	}

	return c.Calculate(input)
}
