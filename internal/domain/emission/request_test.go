package emission

import (
	"testing"
)

// TestValuesRequest_HasUnconditionalDiscount tests the HasUnconditionalDiscount method.
func TestValuesRequest_HasUnconditionalDiscount(t *testing.T) {
	tests := []struct {
		name   string
		values *ValuesRequest
		want   bool
	}{
		{
			name:   "nil values",
			values: nil,
			want:   false,
		},
		{
			name: "zero discount",
			values: &ValuesRequest{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 0,
			},
			want: false,
		},
		{
			name: "positive discount",
			values: &ValuesRequest{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 100.00,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.values.HasUnconditionalDiscount()
			if got != tt.want {
				t.Errorf("HasUnconditionalDiscount() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValuesRequest_HasConditionalDiscount tests the HasConditionalDiscount method.
func TestValuesRequest_HasConditionalDiscount(t *testing.T) {
	tests := []struct {
		name   string
		values *ValuesRequest
		want   bool
	}{
		{
			name:   "nil values",
			values: nil,
			want:   false,
		},
		{
			name: "zero discount",
			values: &ValuesRequest{
				ServiceValue:        1000.00,
				ConditionalDiscount: 0,
			},
			want: false,
		},
		{
			name: "positive discount",
			values: &ValuesRequest{
				ServiceValue:        1000.00,
				ConditionalDiscount: 50.00,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.values.HasConditionalDiscount()
			if got != tt.want {
				t.Errorf("HasConditionalDiscount() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValuesRequest_HasDeductions tests the HasDeductions method.
func TestValuesRequest_HasDeductions(t *testing.T) {
	tests := []struct {
		name   string
		values *ValuesRequest
		want   bool
	}{
		{
			name:   "nil values",
			values: nil,
			want:   false,
		},
		{
			name: "zero deductions",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
				Deductions:   0,
			},
			want: false,
		},
		{
			name: "positive deductions",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
				Deductions:   200.00,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.values.HasDeductions()
			if got != tt.want {
				t.Errorf("HasDeductions() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValuesRequest_HasAnyDiscount tests the HasAnyDiscount method.
func TestValuesRequest_HasAnyDiscount(t *testing.T) {
	tests := []struct {
		name   string
		values *ValuesRequest
		want   bool
	}{
		{
			name:   "nil values",
			values: nil,
			want:   false,
		},
		{
			name: "no discounts",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
			},
			want: false,
		},
		{
			name: "only unconditional discount",
			values: &ValuesRequest{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 50.00,
			},
			want: true,
		},
		{
			name: "only conditional discount",
			values: &ValuesRequest{
				ServiceValue:        1000.00,
				ConditionalDiscount: 50.00,
			},
			want: true,
		},
		{
			name: "only deductions",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
				Deductions:   50.00,
			},
			want: true,
		},
		{
			name: "all discounts",
			values: &ValuesRequest{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 50.00,
				ConditionalDiscount:   25.00,
				Deductions:            100.00,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.values.HasAnyDiscount()
			if got != tt.want {
				t.Errorf("HasAnyDiscount() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValuesRequest_CalculateTaxBase tests the CalculateTaxBase method.
func TestValuesRequest_CalculateTaxBase(t *testing.T) {
	tests := []struct {
		name   string
		values *ValuesRequest
		want   float64
	}{
		{
			name:   "nil values",
			values: nil,
			want:   0,
		},
		{
			name: "no discounts",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
			},
			want: 1000.00,
		},
		{
			name: "with unconditional discount",
			values: &ValuesRequest{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
			},
			want: 1400.00,
		},
		{
			name: "with conditional discount (should NOT affect tax base)",
			values: &ValuesRequest{
				ServiceValue:        1500.00,
				ConditionalDiscount: 100.00,
			},
			want: 1500.00, // Conditional discount does NOT reduce tax base
		},
		{
			name: "with deductions",
			values: &ValuesRequest{
				ServiceValue: 1500.00,
				Deductions:   200.00,
			},
			want: 1300.00,
		},
		{
			name: "complete calculation",
			values: &ValuesRequest{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
				ConditionalDiscount:   50.00, // Does NOT affect tax base
				Deductions:            200.00,
			},
			want: 1200.00, // 1500 - 100 - 200 = 1200
		},
		{
			name: "would be negative - clamp to 0",
			values: &ValuesRequest{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 600.00,
				Deductions:            500.00,
			},
			want: 0, // Clamped to 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.values.CalculateTaxBase()
			if !floatEquals(got, tt.want) {
				t.Errorf("CalculateTaxBase() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

// TestValuesRequest_CalculateDeductionPercentage tests the CalculateDeductionPercentage method.
func TestValuesRequest_CalculateDeductionPercentage(t *testing.T) {
	tests := []struct {
		name   string
		values *ValuesRequest
		want   float64
	}{
		{
			name:   "nil values",
			values: nil,
			want:   0,
		},
		{
			name: "zero service value",
			values: &ValuesRequest{
				ServiceValue: 0,
				Deductions:   100.00,
			},
			want: 0,
		},
		{
			name: "no deductions",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
				Deductions:   0,
			},
			want: 0,
		},
		{
			name: "10% deduction",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
				Deductions:   100.00,
			},
			want: 10.00,
		},
		{
			name: "13.33% deduction",
			values: &ValuesRequest{
				ServiceValue: 1500.00,
				Deductions:   200.00,
			},
			want: 13.33,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.values.CalculateDeductionPercentage()
			if !floatEquals(got, tt.want) {
				t.Errorf("CalculateDeductionPercentage() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

// TestValuesRequest_CalculateNetValue tests the CalculateNetValue method.
func TestValuesRequest_CalculateNetValue(t *testing.T) {
	tests := []struct {
		name   string
		values *ValuesRequest
		want   float64
	}{
		{
			name:   "nil values",
			values: nil,
			want:   0,
		},
		{
			name: "no discounts",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
			},
			want: 1000.00,
		},
		{
			name: "all discounts",
			values: &ValuesRequest{
				ServiceValue:          1500.00,
				UnconditionalDiscount: 100.00,
				ConditionalDiscount:   50.00,
				Deductions:            200.00,
			},
			want: 1150.00, // 1500 - 100 - 50 - 200 = 1150
		},
		{
			name: "would be negative - clamp to 0",
			values: &ValuesRequest{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 400.00,
				ConditionalDiscount:   400.00,
				Deductions:            300.00,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.values.CalculateNetValue()
			if !floatEquals(got, tt.want) {
				t.Errorf("CalculateNetValue() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

// TestValuesRequest_TotalTaxBaseDeductions tests the TotalTaxBaseDeductions method.
func TestValuesRequest_TotalTaxBaseDeductions(t *testing.T) {
	tests := []struct {
		name   string
		values *ValuesRequest
		want   float64
	}{
		{
			name:   "nil values",
			values: nil,
			want:   0,
		},
		{
			name: "no deductions",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
			},
			want: 0,
		},
		{
			name: "only unconditional discount",
			values: &ValuesRequest{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 100.00,
			},
			want: 100.00,
		},
		{
			name: "only deductions",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
				Deductions:   200.00,
			},
			want: 200.00,
		},
		{
			name: "unconditional discount + deductions (not conditional)",
			values: &ValuesRequest{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 100.00,
				ConditionalDiscount:   50.00, // NOT included
				Deductions:            200.00,
			},
			want: 300.00, // 100 + 200 = 300 (conditional NOT included)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.values.TotalTaxBaseDeductions()
			if !floatEquals(got, tt.want) {
				t.Errorf("TotalTaxBaseDeductions() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

// TestValuesRequest_TotalDiscounts tests the TotalDiscounts method.
func TestValuesRequest_TotalDiscounts(t *testing.T) {
	tests := []struct {
		name   string
		values *ValuesRequest
		want   float64
	}{
		{
			name:   "nil values",
			values: nil,
			want:   0,
		},
		{
			name: "no discounts",
			values: &ValuesRequest{
				ServiceValue: 1000.00,
			},
			want: 0,
		},
		{
			name: "all discounts",
			values: &ValuesRequest{
				ServiceValue:          1000.00,
				UnconditionalDiscount: 100.00,
				ConditionalDiscount:   50.00,
				Deductions:            200.00,
			},
			want: 350.00, // 100 + 50 + 200 = 350
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.values.TotalDiscounts()
			if !floatEquals(got, tt.want) {
				t.Errorf("TotalDiscounts() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

// BenchmarkValuesRequest_CalculateTaxBase benchmarks the tax base calculation.
func BenchmarkValuesRequest_CalculateTaxBase(b *testing.B) {
	values := &ValuesRequest{
		ServiceValue:          1500.00,
		UnconditionalDiscount: 100.00,
		ConditionalDiscount:   50.00,
		Deductions:            200.00,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = values.CalculateTaxBase()
	}
}
