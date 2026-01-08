// Package validation provides validation logic for NFS-e domain objects.
package validation

import (
	"fmt"
	"math"

	"github.com/eduardo/nfse-nacional/internal/domain"
)

// Value validation error codes.
const (
	// ValidationCodeInvalidServiceValue indicates an invalid service value.
	ValidationCodeInvalidServiceValue = "INVALID_SERVICE_VALUE"

	// ValidationCodeInvalidDiscount indicates an invalid discount value.
	ValidationCodeInvalidDiscount = "INVALID_DISCOUNT"

	// ValidationCodeDiscountExceedsValue indicates discount exceeds service value.
	ValidationCodeDiscountExceedsValue = "DISCOUNT_EXCEEDS_VALUE"

	// ValidationCodeInvalidDeduction indicates an invalid deduction value.
	ValidationCodeInvalidDeduction = "INVALID_DEDUCTION"

	// ValidationCodeDeductionExceedsValue indicates deduction exceeds service value.
	ValidationCodeDeductionExceedsValue = "DEDUCTION_EXCEEDS_VALUE"

	// ValidationCodeNegativeTaxBase indicates the calculated tax base would be negative.
	ValidationCodeNegativeTaxBase = "NEGATIVE_TAX_BASE"
)

// MaxServiceValue is the maximum allowed service value (999,999,999.99).
const MaxServiceValue = 999999999.99

// ValuesValidator validates the monetary values section of an NFS-e emission request.
// It enforces Brazilian NFS-e rules for discounts, deductions, and tax base calculation.
type ValuesValidator struct{}

// NewValuesValidator creates a new ValuesValidator.
func NewValuesValidator() *ValuesValidator {
	return &ValuesValidator{}
}

// ValidateValues validates the values section of an emission request.
// It returns a slice of ValidationError if any validation fails.
//
// Validation rules:
// - service_value: required, > 0, max 2 decimals, max 999999999.99
// - unconditional_discount: optional, >= 0, max 2 decimals, <= service_value
// - conditional_discount: optional, >= 0, max 2 decimals, <= service_value
// - deductions: optional, >= 0, max 2 decimals
// - unconditional_discount + deductions <= service_value (tax base cannot be negative)
func (v *ValuesValidator) ValidateValues(values *domain.Values) []ValidationError {
	if values == nil {
		return []ValidationError{
			NewValidationError(
				"values",
				ValidationCodeRequired,
				"Values section is required",
			),
		}
	}

	var errors []ValidationError

	// Validate service value
	errors = append(errors, v.validateServiceValue(values.ServiceValue)...)

	// Validate unconditional discount
	errors = append(errors, v.validateUnconditionalDiscount(values.ServiceValue, values.UnconditionalDiscount)...)

	// Validate conditional discount
	errors = append(errors, v.validateConditionalDiscount(values.ServiceValue, values.ConditionalDiscount)...)

	// Validate deductions
	errors = append(errors, v.validateDeductions(values.ServiceValue, values.Deductions)...)

	// Validate that tax base would not be negative
	// Tax base = ServiceValue - UnconditionalDiscount - Deductions
	// (ConditionalDiscount does NOT affect tax base)
	if len(errors) == 0 {
		errors = append(errors, v.validateTaxBase(values)...)
	}

	return errors
}

// validateServiceValue validates the service value field.
func (v *ValuesValidator) validateServiceValue(value float64) []ValidationError {
	var errors []ValidationError

	// Service value is required and must be > 0
	if value <= 0 {
		errors = append(errors, NewValidationError(
			"values.service_value",
			ValidationCodeInvalidServiceValue,
			"Service value must be greater than zero",
		))
		return errors
	}

	// Check maximum value
	if value > MaxServiceValue {
		errors = append(errors, NewValidationError(
			"values.service_value",
			ValidationCodeInvalidServiceValue,
			fmt.Sprintf("Service value cannot exceed %.2f", MaxServiceValue),
		))
	}

	// Check decimal precision (max 2 decimal places)
	if !hasValidDecimalPrecision(value) {
		errors = append(errors, NewValidationError(
			"values.service_value",
			ValidationCodeInvalidServiceValue,
			"Service value must have at most 2 decimal places",
		))
	}

	return errors
}

// validateUnconditionalDiscount validates the unconditional discount field.
func (v *ValuesValidator) validateUnconditionalDiscount(serviceValue, discount float64) []ValidationError {
	var errors []ValidationError

	// Skip validation if no discount
	if discount == 0 {
		return errors
	}

	// Unconditional discount cannot be negative
	if discount < 0 {
		errors = append(errors, NewValidationError(
			"values.unconditional_discount",
			ValidationCodeInvalidDiscount,
			"Unconditional discount cannot be negative",
		))
		return errors
	}

	// Check decimal precision (max 2 decimal places)
	if !hasValidDecimalPrecision(discount) {
		errors = append(errors, NewValidationError(
			"values.unconditional_discount",
			ValidationCodeInvalidDiscount,
			"Unconditional discount must have at most 2 decimal places",
		))
	}

	// Unconditional discount cannot exceed service value
	if discount > serviceValue {
		errors = append(errors, NewValidationError(
			"values.unconditional_discount",
			ValidationCodeDiscountExceedsValue,
			fmt.Sprintf("Unconditional discount (%.2f) cannot exceed service value (%.2f)",
				discount, serviceValue),
		))
	}

	return errors
}

// validateConditionalDiscount validates the conditional discount field.
func (v *ValuesValidator) validateConditionalDiscount(serviceValue, discount float64) []ValidationError {
	var errors []ValidationError

	// Skip validation if no discount
	if discount == 0 {
		return errors
	}

	// Conditional discount cannot be negative
	if discount < 0 {
		errors = append(errors, NewValidationError(
			"values.conditional_discount",
			ValidationCodeInvalidDiscount,
			"Conditional discount cannot be negative",
		))
		return errors
	}

	// Check decimal precision (max 2 decimal places)
	if !hasValidDecimalPrecision(discount) {
		errors = append(errors, NewValidationError(
			"values.conditional_discount",
			ValidationCodeInvalidDiscount,
			"Conditional discount must have at most 2 decimal places",
		))
	}

	// Conditional discount cannot exceed service value
	if discount > serviceValue {
		errors = append(errors, NewValidationError(
			"values.conditional_discount",
			ValidationCodeDiscountExceedsValue,
			fmt.Sprintf("Conditional discount (%.2f) cannot exceed service value (%.2f)",
				discount, serviceValue),
		))
	}

	return errors
}

// validateDeductions validates the deductions field.
func (v *ValuesValidator) validateDeductions(serviceValue, deductions float64) []ValidationError {
	var errors []ValidationError

	// Skip validation if no deductions
	if deductions == 0 {
		return errors
	}

	// Deductions cannot be negative
	if deductions < 0 {
		errors = append(errors, NewValidationError(
			"values.deductions",
			ValidationCodeInvalidDeduction,
			"Deductions cannot be negative",
		))
		return errors
	}

	// Check decimal precision (max 2 decimal places)
	if !hasValidDecimalPrecision(deductions) {
		errors = append(errors, NewValidationError(
			"values.deductions",
			ValidationCodeInvalidDeduction,
			"Deductions must have at most 2 decimal places",
		))
	}

	// Deductions cannot exceed service value
	if deductions > serviceValue {
		errors = append(errors, NewValidationError(
			"values.deductions",
			ValidationCodeDeductionExceedsValue,
			fmt.Sprintf("Deductions (%.2f) cannot exceed service value (%.2f)",
				deductions, serviceValue),
		))
	}

	return errors
}

// validateTaxBase validates that the tax base calculation would not result in a negative value.
// Tax base = ServiceValue - UnconditionalDiscount - Deductions
// Note: ConditionalDiscount does NOT affect the tax base in Brazilian NFS-e rules.
func (v *ValuesValidator) validateTaxBase(values *domain.Values) []ValidationError {
	var errors []ValidationError

	// Calculate the tax base deductions (unconditional discount + deductions)
	taxBaseDeductions := values.UnconditionalDiscount + values.Deductions

	// Tax base cannot be negative
	if taxBaseDeductions > values.ServiceValue {
		errors = append(errors, NewValidationError(
			"values",
			ValidationCodeNegativeTaxBase,
			fmt.Sprintf("Tax base would be negative: unconditional discount (%.2f) plus deductions (%.2f) exceed service value (%.2f)",
				values.UnconditionalDiscount, values.Deductions, values.ServiceValue),
		))
	}

	return errors
}

// CalculateTaxBase calculates the tax base from the given values.
// Tax base = ServiceValue - UnconditionalDiscount - Deductions
// Returns the calculated tax base (never negative, clamped to 0 if calculation yields negative).
func (v *ValuesValidator) CalculateTaxBase(values *domain.Values) float64 {
	if values == nil {
		return 0
	}

	taxBase := values.ServiceValue - values.UnconditionalDiscount - values.Deductions

	// Clamp to 0 if negative (should not happen if validation passed)
	if taxBase < 0 {
		return 0
	}

	return roundToTwoDecimals(taxBase)
}

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
func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}
