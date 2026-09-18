package xmlbuilder

import (
	"strings"
	"testing"
	"time"
)

// TestDPSBuilder_BuildValues_NoDiscounts tests XML generation without discounts.
func TestDPSBuilder_BuildValues_NoDiscounts(t *testing.T) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue: 1000.00,
		ISSRate:      2.00,
	}

	builder := NewDPSBuilder(config)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check service value is present
	if !strings.Contains(result.XML, "<vServ>1000.00</vServ>") {
		t.Error("expected vServ element with value 1000.00")
	}

	// Check no discount elements are present
	if strings.Contains(result.XML, "<vDescIncond>") {
		t.Error("unexpected vDescIncond element")
	}
	if strings.Contains(result.XML, "<vDescCond>") {
		t.Error("unexpected vDescCond element")
	}
	if strings.Contains(result.XML, "<vDedRed>") {
		t.Error("unexpected vDedRed element")
	}

	// Check tax section is present
	if !strings.Contains(result.XML, "<trib>") {
		t.Error("expected trib element")
	}
	if !strings.Contains(result.XML, "<vBCCalc>1000.00</vBCCalc>") {
		t.Error("expected vBCCalc element with value 1000.00")
	}
	if !strings.Contains(result.XML, "<pAliq>2.00</pAliq>") {
		t.Error("expected pAliq element with value 2.00")
	}
	if !strings.Contains(result.XML, "<vISS>20.00</vISS>") {
		t.Error("expected vISS element with value 20.00")
	}
}

// TestDPSBuilder_BuildValues_WithUnconditionalDiscount tests XML generation with unconditional discount.
func TestDPSBuilder_BuildValues_WithUnconditionalDiscount(t *testing.T) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue:          1500.00,
		UnconditionalDiscount: 100.00,
		ISSRate:               2.00,
	}

	builder := NewDPSBuilder(config)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check service value
	if !strings.Contains(result.XML, "<vServ>1500.00</vServ>") {
		t.Error("expected vServ element with value 1500.00")
	}

	// Check unconditional discount
	if !strings.Contains(result.XML, "<vDescIncond>100.00</vDescIncond>") {
		t.Error("expected vDescIncond element with value 100.00")
	}

	// Check tax base is reduced by unconditional discount
	// Tax base = 1500 - 100 = 1400
	if !strings.Contains(result.XML, "<vBCCalc>1400.00</vBCCalc>") {
		t.Error("expected vBCCalc element with value 1400.00")
	}

	// Check ISS = 1400 * 2% = 28
	if !strings.Contains(result.XML, "<vISS>28.00</vISS>") {
		t.Error("expected vISS element with value 28.00")
	}
}

// TestDPSBuilder_BuildValues_WithConditionalDiscount tests that conditional discount
// does NOT affect the tax base.
func TestDPSBuilder_BuildValues_WithConditionalDiscount(t *testing.T) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue:        1500.00,
		ConditionalDiscount: 100.00,
		ISSRate:             2.00,
	}

	builder := NewDPSBuilder(config)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check conditional discount is present in XML
	if !strings.Contains(result.XML, "<vDescCond>100.00</vDescCond>") {
		t.Error("expected vDescCond element with value 100.00")
	}

	// IMPORTANT: Tax base should NOT be reduced by conditional discount
	// Tax base = 1500 (NOT 1400)
	if !strings.Contains(result.XML, "<vBCCalc>1500.00</vBCCalc>") {
		t.Error("expected vBCCalc element with value 1500.00 (conditional discount should NOT reduce tax base)")
	}

	// Check ISS = 1500 * 2% = 30
	if !strings.Contains(result.XML, "<vISS>30.00</vISS>") {
		t.Error("expected vISS element with value 30.00")
	}
}

// TestDPSBuilder_BuildValues_WithDeductions tests XML generation with deductions.
func TestDPSBuilder_BuildValues_WithDeductions(t *testing.T) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue: 1500.00,
		Deductions:   200.00,
		ISSRate:      2.00,
	}

	builder := NewDPSBuilder(config)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check deduction section is present
	if !strings.Contains(result.XML, "<vDedRed>") {
		t.Error("expected vDedRed element")
	}
	if !strings.Contains(result.XML, "<vDR>200.00</vDR>") {
		t.Error("expected vDR element with value 200.00")
	}

	// Check deduction percentage = (200/1500) * 100 = 13.33
	if !strings.Contains(result.XML, "<pDR>13.33</pDR>") {
		t.Error("expected pDR element with value 13.33")
	}

	// Check tax base is reduced by deductions
	// Tax base = 1500 - 200 = 1300
	if !strings.Contains(result.XML, "<vBCCalc>1300.00</vBCCalc>") {
		t.Error("expected vBCCalc element with value 1300.00")
	}

	// Check ISS = 1300 * 2% = 26
	if !strings.Contains(result.XML, "<vISS>26.00</vISS>") {
		t.Error("expected vISS element with value 26.00")
	}
}

// TestDPSBuilder_BuildValues_Complete tests XML generation with all discounts and deductions.
func TestDPSBuilder_BuildValues_Complete(t *testing.T) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue:          1500.00,
		UnconditionalDiscount: 100.00,
		ConditionalDiscount:   50.00,
		Deductions:            200.00,
		ISSRate:               2.00,
	}

	builder := NewDPSBuilder(config)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check all values are present
	if !strings.Contains(result.XML, "<vServ>1500.00</vServ>") {
		t.Error("expected vServ element with value 1500.00")
	}
	if !strings.Contains(result.XML, "<vDescIncond>100.00</vDescIncond>") {
		t.Error("expected vDescIncond element with value 100.00")
	}
	if !strings.Contains(result.XML, "<vDescCond>50.00</vDescCond>") {
		t.Error("expected vDescCond element with value 50.00")
	}
	if !strings.Contains(result.XML, "<vDR>200.00</vDR>") {
		t.Error("expected vDR element with value 200.00")
	}

	// Check tax base calculation: 1500 - 100 - 200 = 1200
	// (conditional discount of 50 does NOT reduce tax base)
	if !strings.Contains(result.XML, "<vBCCalc>1200.00</vBCCalc>") {
		t.Error("expected vBCCalc element with value 1200.00")
	}

	// Check ISS = 1200 * 2% = 24
	if !strings.Contains(result.XML, "<vISS>24.00</vISS>") {
		t.Error("expected vISS element with value 24.00")
	}
}

// TestDPSBuilder_BuildValues_MEI_NoISS tests XML generation for MEI (0% ISS rate).
func TestDPSBuilder_BuildValues_MEI_NoISS(t *testing.T) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue:          1000.00,
		UnconditionalDiscount: 50.00,
		ISSRate:               0, // MEI
	}

	builder := NewDPSBuilder(config)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check tax base is still calculated correctly
	if !strings.Contains(result.XML, "<vBCCalc>950.00</vBCCalc>") {
		t.Error("expected vBCCalc element with value 950.00")
	}

	// Check ISS rate is 0
	if !strings.Contains(result.XML, "<pAliq>0.00</pAliq>") {
		t.Error("expected pAliq element with value 0.00")
	}

	// Check ISS amount is 0
	if !strings.Contains(result.XML, "<vISS>0.00</vISS>") {
		t.Error("expected vISS element with value 0.00")
	}
}

// TestDPSBuilder_BuildValues_PreCalculatedValues tests using pre-calculated values.
func TestDPSBuilder_BuildValues_PreCalculatedValues(t *testing.T) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue:          1500.00,
		UnconditionalDiscount: 100.00,
		Deductions:            200.00,
		DeductionPercentage:   13.33,
		TaxBase:               1200.00,
		ISSRate:               2.00,
		ISSAmount:             24.00,
	}

	builder := NewDPSBuilder(config)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check pre-calculated values are used
	if !strings.Contains(result.XML, "<pDR>13.33</pDR>") {
		t.Error("expected pDR element with pre-calculated value 13.33")
	}
	if !strings.Contains(result.XML, "<vBCCalc>1200.00</vBCCalc>") {
		t.Error("expected vBCCalc element with pre-calculated value 1200.00")
	}
	if !strings.Contains(result.XML, "<vISS>24.00</vISS>") {
		t.Error("expected vISS element with pre-calculated value 24.00")
	}
}

// TestDPSBuilder_BuildValues_TotalTribSection tests the totTrib section generation.
func TestDPSBuilder_BuildValues_TotalTribSection(t *testing.T) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue: 1000.00,
	}

	builder := NewDPSBuilder(config)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check totTrib section is present
	if !strings.Contains(result.XML, "<totTrib>") {
		t.Error("expected totTrib element")
	}
	if !strings.Contains(result.XML, "<indTotTrib>0</indTotTrib>") {
		t.Error("expected indTotTrib element with value 0")
	}
	if !strings.Contains(result.XML, "<pTotTribFed>0.00</pTotTribFed>") {
		t.Error("expected pTotTribFed element with value 0.00")
	}
	if !strings.Contains(result.XML, "<pTotTribEst>0.00</pTotTribEst>") {
		t.Error("expected pTotTribEst element with value 0.00")
	}
	if !strings.Contains(result.XML, "<pTotTribMun>0.00</pTotTribMun>") {
		t.Error("expected pTotTribMun element with value 0.00")
	}
}

// TestDPSBuilder_BuildValues_XMLStructure tests the overall XML structure.
func TestDPSBuilder_BuildValues_XMLStructure(t *testing.T) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue:          1500.00,
		UnconditionalDiscount: 100.00,
		ConditionalDiscount:   50.00,
		Deductions:            200.00,
		ISSRate:               2.00,
	}

	builder := NewDPSBuilder(config)
	result, err := builder.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check XML structure elements are in correct order
	// valores should contain: vServPrest, vDedRed (optional), trib, totTrib
	valoresIdx := strings.Index(result.XML, "<valores>")
	vServPrestIdx := strings.Index(result.XML, "<vServPrest>")
	vDedRedIdx := strings.Index(result.XML, "<vDedRed>")
	tribIdx := strings.Index(result.XML, "<trib>")
	totTribIdx := strings.Index(result.XML, "<totTrib>")

	if valoresIdx == -1 {
		t.Fatal("expected valores element")
	}
	if vServPrestIdx == -1 || vServPrestIdx < valoresIdx {
		t.Error("expected vServPrest element inside valores")
	}
	if vDedRedIdx == -1 || vDedRedIdx < vServPrestIdx {
		t.Error("expected vDedRed element after vServPrest")
	}
	if tribIdx == -1 || tribIdx < vDedRedIdx {
		t.Error("expected trib element after vDedRed")
	}
	if totTribIdx == -1 || totTribIdx < tribIdx {
		t.Error("expected totTrib element after trib")
	}
}

// createBasicDPSConfig creates a basic DPS configuration for testing.
func createBasicDPSConfig() DPSConfig {
	return DPSConfig{
		Environment:        2, // Homologation
		EmissionDateTime:   time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		ApplicationVersion: "1.0.0",
		Series:             "00001",
		Number:             "123",
		CompetenceDate:     time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		EmitterType:        1,
		MunicipalityCode:   "3550308", // Sao Paulo
		Substitution:       2,
		Provider: DPSProvider{
			CNPJ:      "12345678000190",
			Name:      "Test Provider Ltda",
			TaxRegime: "mei",
		},
		Service: DPSService{
			NationalCode:     "123456",
			Description:      "Test service",
			MunicipalityCode: "3550308",
		},
	}
}

// BenchmarkDPSBuilder_Build_WithDiscounts benchmarks XML generation with discounts.
func BenchmarkDPSBuilder_Build_WithDiscounts(b *testing.B) {
	config := createBasicDPSConfig()
	config.Values = DPSValues{
		ServiceValue:          1500.00,
		UnconditionalDiscount: 100.00,
		ConditionalDiscount:   50.00,
		Deductions:            200.00,
		ISSRate:               2.00,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewDPSBuilder(config)
		_, _ = builder.Build()
	}
}
