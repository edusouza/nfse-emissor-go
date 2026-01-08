package validation

import (
	"math"
	"testing"

	"github.com/eduardo/nfse-nacional/internal/domain"
)

// TestValuesValidator_ValidateValues tests the main validation method.
func TestValuesValidator_ValidateValues(t *testing.T) {
	validator := NewValuesValidator()

	tests := []struct {
		name       string
		values     *domain.Values
		wantErrors int
		wantCodes  []string
	}{
		{
			name: "valid values - no discounts",
			values: &domain.Values{
				ServiceValue: 1000.00,
			},
			wantErrors: 0,
		},
		{
			name: "valid values - with unconditional discount",
			values: &domain.Values{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
			},
			wantErrors: 0,
		},
		{
			name: "valid values - with conditional discount",
			values: &domain.Values{
				ServiceValue:        1500.00,
				ConditionalDiscount: 100.00,
			},
			wantErrors: 0,
		},
		{
			name: "valid values - with deductions",
			values: &domain.Values{
				ServiceValue: 1500.00,
				Deductions:   200.00,
			},
			wantErrors: 0,
		},
		{
			name: "valid values - all fields",
			values: &domain.Values{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
				ConditionalDiscount:   50.00,
				Deductions:            200.00,
			},
			wantErrors: 0,
		},
		{
			name:       "nil values",
			values:     nil,
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeRequired},
		},
		{
			name: "zero service value",
			values: &domain.Values{
				ServiceValue: 0,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeInvalidServiceValue},
		},
		{
			name: "negative service value",
			values: &domain.Values{
				ServiceValue: -100.00,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeInvalidServiceValue},
		},
		{
			name: "service value exceeds maximum",
			values: &domain.Values{
				ServiceValue: 1000000000.00,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeInvalidServiceValue},
		},
		{
			name: "service value invalid decimal precision",
			values: &domain.Values{
				ServiceValue: 100.123,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeInvalidServiceValue},
		},
		{
			name: "negative unconditional discount",
			values: &domain.Values{
				ServiceValue:          1000.00,
				UnconditionalDiscount: -50.00,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeInvalidDiscount},
		},
		{
			name: "unconditional discount exceeds service value",
			values: &domain.Values{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 1100.00,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeDiscountExceedsValue},
		},
		{
			name: "unconditional discount invalid decimal precision",
			values: &domain.Values{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 50.123,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeInvalidDiscount},
		},
		{
			name: "negative conditional discount",
			values: &domain.Values{
				ServiceValue:        1000.00,
				ConditionalDiscount: -50.00,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeInvalidDiscount},
		},
		{
			name: "conditional discount exceeds service value",
			values: &domain.Values{
				ServiceValue:        1000.00,
				ConditionalDiscount: 1100.00,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeDiscountExceedsValue},
		},
		{
			name: "negative deductions",
			values: &domain.Values{
				ServiceValue: 1000.00,
				Deductions:   -50.00,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeInvalidDeduction},
		},
		{
			name: "deductions exceed service value",
			values: &domain.Values{
				ServiceValue: 1000.00,
				Deductions:   1100.00,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeDeductionExceedsValue},
		},
		{
			name: "deductions invalid decimal precision",
			values: &domain.Values{
				ServiceValue: 1000.00,
				Deductions:   50.123,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeInvalidDeduction},
		},
		{
			name: "negative tax base - unconditional + deductions exceed service value",
			values: &domain.Values{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 600.00,
				Deductions:            500.00,
			},
			wantErrors: 1,
			wantCodes:  []string{ValidationCodeNegativeTaxBase},
		},
		{
			name: "tax base valid even with large conditional discount",
			values: &domain.Values{
				ServiceValue:          1000.00,
				ConditionalDiscount:   900.00, // Large but does NOT affect tax base
				UnconditionalDiscount: 50.00,
				Deductions:            50.00,
			},
			wantErrors: 0, // Valid because conditional discount doesn't reduce tax base
		},
		{
			name: "multiple errors",
			values: &domain.Values{
				ServiceValue:          0,
				UnconditionalDiscount: -10.00,
			},
			wantErrors: 2,
			wantCodes:  []string{ValidationCodeInvalidServiceValue, ValidationCodeInvalidDiscount},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateValues(tt.values)

			if len(errors) != tt.wantErrors {
				t.Errorf("got %d errors, want %d", len(errors), tt.wantErrors)
				for _, err := range errors {
					t.Logf("  error: %s - %s", err.Code, err.Message)
				}
			}

			if tt.wantCodes != nil {
				for i, wantCode := range tt.wantCodes {
					if i < len(errors) && errors[i].Code != wantCode {
						t.Errorf("error[%d] code = %s, want %s", i, errors[i].Code, wantCode)
					}
				}
			}
		})
	}
}

// TestValuesValidator_CalculateTaxBase tests the tax base calculation method.
func TestValuesValidator_CalculateTaxBase(t *testing.T) {
	validator := NewValuesValidator()

	tests := []struct {
		name   string
		values *domain.Values
		want   float64
	}{
		{
			name:   "nil values",
			values: nil,
			want:   0,
		},
		{
			name: "no discounts",
			values: &domain.Values{
				ServiceValue: 1000.00,
			},
			want: 1000.00,
		},
		{
			name: "with unconditional discount",
			values: &domain.Values{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
			},
			want: 1400.00,
		},
		{
			name: "with conditional discount (should NOT affect tax base)",
			values: &domain.Values{
				ServiceValue:        1500.00,
				ConditionalDiscount: 100.00,
			},
			want: 1500.00, // Conditional discount does NOT reduce tax base
		},
		{
			name: "with deductions",
			values: &domain.Values{
				ServiceValue: 1500.00,
				Deductions:   200.00,
			},
			want: 1300.00,
		},
		{
			name: "all discounts and deductions",
			values: &domain.Values{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
				ConditionalDiscount:   50.00, // Does NOT reduce tax base
				Deductions:            200.00,
			},
			want: 1200.00, // 1500 - 100 - 200 = 1200
		},
		{
			name: "would be negative - clamp to 0",
			values: &domain.Values{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 600.00,
				Deductions:            500.00,
			},
			want: 0, // Clamped to 0 instead of -100
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validator.CalculateTaxBase(tt.values)
			if !floatEquals(got, tt.want) {
				t.Errorf("CalculateTaxBase() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

// TestHasValidDecimalPrecision tests the decimal precision validation.
func TestHasValidDecimalPrecision(t *testing.T) {
	tests := []struct {
		value float64
		want  bool
	}{
		{100.00, true},
		{100.12, true},
		{100.99, true},
		{0.01, true},
		{0.10, true},
		{999999999.99, true},
		{100.123, false},
		{100.001, false},
		{0.001, false},
		{100.1234, false},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := hasValidDecimalPrecision(tt.value)
			if got != tt.want {
				t.Errorf("hasValidDecimalPrecision(%.4f) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestRoundToTwoDecimals tests the rounding function.
func TestRoundToTwoDecimals(t *testing.T) {
	tests := []struct {
		value float64
		want  float64
	}{
		{100.125, 100.13},
		{100.124, 100.12},
		{100.1, 100.10},
		{100.999, 101.00},
		{0.001, 0.00},
		{0.009, 0.01},
		{0.005, 0.01}, // Banker's rounding (round half to even)
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := roundToTwoDecimals(tt.value)
			if !floatEquals(got, tt.want) {
				t.Errorf("roundToTwoDecimals(%.4f) = %.2f, want %.2f", tt.value, got, tt.want)
			}
		})
	}
}

// floatEquals compares two float64 values with tolerance for floating point errors.
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}

// BenchmarkValuesValidator_ValidateValues benchmarks the validation performance.
func BenchmarkValuesValidator_ValidateValues(b *testing.B) {
	validator := NewValuesValidator()
	values := &domain.Values{
		ServiceValue:          1500.00,
		UnconditionalDiscount: 100.00,
		ConditionalDiscount:   50.00,
		Deductions:            200.00,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = validator.ValidateValues(values)
	}
}
