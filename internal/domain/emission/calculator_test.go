package emission

import (
	"math"
	"testing"
)

// TestValueCalculator_Calculate tests the main Calculate method.
func TestValueCalculator_Calculate(t *testing.T) {
	calculator := NewValueCalculator()

	tests := []struct {
		name        string
		input       *CalculationInput
		wantErr     bool
		errCode     string
		wantTaxBase float64
		wantISS     float64
		wantDedPct  float64
	}{
		{
			name: "basic calculation without discounts",
			input: &CalculationInput{
				ServiceValue: 1000.00,
				ISSRate:      2.00,
			},
			wantErr:     false,
			wantTaxBase: 1000.00,
			wantISS:     20.00,
			wantDedPct:  0,
		},
		{
			name: "calculation with unconditional discount",
			input: &CalculationInput{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
				ISSRate:               2.00,
			},
			wantErr:     false,
			wantTaxBase: 1400.00,
			wantISS:     28.00,
			wantDedPct:  0,
		},
		{
			name: "calculation with conditional discount (should NOT affect tax base)",
			input: &CalculationInput{
				ServiceValue:        1500.00,
				ConditionalDiscount: 100.00,
				ISSRate:             2.00,
			},
			wantErr:     false,
			wantTaxBase: 1500.00, // Conditional discount does NOT reduce tax base
			wantISS:     30.00,
			wantDedPct:  0,
		},
		{
			name: "calculation with deductions",
			input: &CalculationInput{
				ServiceValue: 1500.00,
				Deductions:   200.00,
				ISSRate:      2.00,
			},
			wantErr:     false,
			wantTaxBase: 1300.00,
			wantISS:     26.00,
			wantDedPct:  13.33, // (200/1500) * 100 = 13.33...
		},
		{
			name: "complete calculation with all discounts/deductions",
			input: &CalculationInput{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
				ConditionalDiscount:   50.00,
				Deductions:            200.00,
				ISSRate:               2.00,
			},
			wantErr:     false,
			wantTaxBase: 1200.00, // 1500 - 100 - 200 = 1200 (conditional discount NOT subtracted)
			wantISS:     24.00,   // 1200 * 2% = 24
			wantDedPct:  13.33,   // (200/1500) * 100
		},
		{
			name: "MEI scenario with 0% ISS rate",
			input: &CalculationInput{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 50.00,
				ISSRate:               0,
			},
			wantErr:     false,
			wantTaxBase: 950.00,
			wantISS:     0,
			wantDedPct:  0,
		},
		{
			name: "zero service value",
			input: &CalculationInput{
				ServiceValue: 0,
			},
			wantErr: true,
			errCode: ErrCodeInvalidServiceValue,
		},
		{
			name: "negative service value",
			input: &CalculationInput{
				ServiceValue: -100.00,
			},
			wantErr: true,
			errCode: ErrCodeInvalidServiceValue,
		},
		{
			name: "service value exceeds maximum",
			input: &CalculationInput{
				ServiceValue: 1000000000.00, // > 999999999.99
			},
			wantErr: true,
			errCode: ErrCodeInvalidServiceValue,
		},
		{
			name: "unconditional discount exceeds service value",
			input: &CalculationInput{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 1100.00,
			},
			wantErr: true,
			errCode: ErrCodeDiscountExceedsValue,
		},
		{
			name: "conditional discount exceeds service value",
			input: &CalculationInput{
				ServiceValue:        1000.00,
				ConditionalDiscount: 1100.00,
			},
			wantErr: true,
			errCode: ErrCodeDiscountExceedsValue,
		},
		{
			name: "deductions exceed service value",
			input: &CalculationInput{
				ServiceValue: 1000.00,
				Deductions:   1100.00,
			},
			wantErr: true,
			errCode: ErrCodeDeductionExceedsValue,
		},
		{
			name: "negative unconditional discount",
			input: &CalculationInput{
				ServiceValue:          1000.00,
				UnconditionalDiscount: -50.00,
			},
			wantErr: true,
			errCode: ErrCodeInvalidDiscount,
		},
		{
			name: "negative conditional discount",
			input: &CalculationInput{
				ServiceValue:        1000.00,
				ConditionalDiscount: -50.00,
			},
			wantErr: true,
			errCode: ErrCodeInvalidDiscount,
		},
		{
			name: "negative deductions",
			input: &CalculationInput{
				ServiceValue: 1000.00,
				Deductions:   -50.00,
			},
			wantErr: true,
			errCode: ErrCodeInvalidDeduction,
		},
		{
			name: "negative tax base (unconditional + deductions exceed service value)",
			input: &CalculationInput{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 600.00,
				Deductions:            500.00,
			},
			wantErr: true,
			errCode: ErrCodeNegativeTaxBase,
		},
		{
			name: "invalid decimal precision - service value",
			input: &CalculationInput{
				ServiceValue: 100.123, // 3 decimal places
			},
			wantErr: true,
			errCode: ErrCodeInvalidServiceValue,
		},
		{
			name: "invalid decimal precision - unconditional discount",
			input: &CalculationInput{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 50.123, // 3 decimal places
			},
			wantErr: true,
			errCode: ErrCodeInvalidDiscount,
		},
		{
			name: "valid decimal precision - 2 decimal places",
			input: &CalculationInput{
				ServiceValue:          1000.99,
				UnconditionalDiscount: 50.50,
				Deductions:            25.25,
				ISSRate:               2.50,
			},
			wantErr:     false,
			wantTaxBase: 925.24,
			wantISS:     23.13, // 925.24 * 2.5% = 23.131 rounded to 23.13
			wantDedPct:  2.52,  // (25.25/1000.99) * 100 = 2.522...
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculator.Calculate(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
					return
				}
				calcErr, ok := err.(*CalculationError)
				if !ok {
					t.Errorf("expected CalculationError, got %T", err)
					return
				}
				if calcErr.Code != tt.errCode {
					t.Errorf("expected error code %s, got %s", tt.errCode, calcErr.Code)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result == nil {
				t.Error("expected result, got nil")
				return
			}

			if !floatEquals(result.TaxBase, tt.wantTaxBase) {
				t.Errorf("TaxBase = %.2f, want %.2f", result.TaxBase, tt.wantTaxBase)
			}

			if !floatEquals(result.ISSAmount, tt.wantISS) {
				t.Errorf("ISSAmount = %.2f, want %.2f", result.ISSAmount, tt.wantISS)
			}

			if !floatEquals(result.DeductionPercentage, tt.wantDedPct) {
				t.Errorf("DeductionPercentage = %.2f, want %.2f", result.DeductionPercentage, tt.wantDedPct)
			}
		})
	}
}

// TestValueCalculator_Calculate_NilInput tests nil input handling.
func TestValueCalculator_Calculate_NilInput(t *testing.T) {
	calculator := NewValueCalculator()

	_, err := calculator.Calculate(nil)
	if err == nil {
		t.Error("expected error for nil input, got nil")
	}
}

// TestValueCalculator_CalculateFromRequest tests the convenience method.
func TestValueCalculator_CalculateFromRequest(t *testing.T) {
	calculator := NewValueCalculator()

	tests := []struct {
		name        string
		values      *ValuesRequest
		wantErr     bool
		wantTaxBase float64
	}{
		{
			name: "basic request",
			values: &ValuesRequest{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
				ConditionalDiscount:   50.00,
				Deductions:            200.00,
			},
			wantErr:     false,
			wantTaxBase: 1200.00,
		},
		{
			name:    "nil request",
			values:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculator.CalculateFromRequest(tt.values)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if !floatEquals(result.TaxBase, tt.wantTaxBase) {
				t.Errorf("TaxBase = %.2f, want %.2f", result.TaxBase, tt.wantTaxBase)
			}

			// MEI scenario: ISS should be 0
			if result.ISSAmount != 0 {
				t.Errorf("ISSAmount = %.2f, want 0 (MEI scenario)", result.ISSAmount)
			}
		})
	}
}

// TestValueCalculator_CalculateFromRequestWithRate tests the rate-specific convenience method.
func TestValueCalculator_CalculateFromRequestWithRate(t *testing.T) {
	calculator := NewValueCalculator()

	values := &ValuesRequest{
		ServiceValue:          1000.00,
		UnconditionalDiscount: 100.00,
	}

	result, err := calculator.CalculateFromRequestWithRate(values, 5.0)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// Tax base = 1000 - 100 = 900
	if !floatEquals(result.TaxBase, 900.00) {
		t.Errorf("TaxBase = %.2f, want 900.00", result.TaxBase)
	}

	// ISS = 900 * 5% = 45
	if !floatEquals(result.ISSAmount, 45.00) {
		t.Errorf("ISSAmount = %.2f, want 45.00", result.ISSAmount)
	}
}

// TestHasValidDecimalPrecision tests the decimal precision validation helper.
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
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := hasValidDecimalPrecision(tt.value)
			if got != tt.want {
				t.Errorf("hasValidDecimalPrecision(%.3f) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// TestRoundToTwoDecimals tests the rounding helper.
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
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := roundToTwoDecimals(tt.value)
			if !floatEquals(got, tt.want) {
				t.Errorf("roundToTwoDecimals(%.3f) = %.2f, want %.2f", tt.value, got, tt.want)
			}
		})
	}
}

// floatEquals compares two float64 values with a small tolerance for floating point errors.
func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}

// BenchmarkValueCalculator_Calculate benchmarks the calculation performance.
func BenchmarkValueCalculator_Calculate(b *testing.B) {
	calculator := NewValueCalculator()
	input := &CalculationInput{
		ServiceValue:          1500.00,
		UnconditionalDiscount: 100.00,
		ConditionalDiscount:   50.00,
		Deductions:            200.00,
		ISSRate:               2.00,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = calculator.Calculate(input)
	}
}
