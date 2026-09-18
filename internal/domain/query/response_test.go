package query

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNFSeQueryResponse_JSONSerialization(t *testing.T) {
	doc := "98765432000188"
	aliquota := 5.00
	valorISSQN := 50.00

	response := &NFSeQueryResponse{
		ChaveAcesso: "NFSe35503082024010112345678000199000010000000001",
		Numero:      "000001",
		DataEmissao: "2024-01-01T10:30:00Z",
		Status:      NFSeStatusActive,
		Prestador: PrestadorInfo{
			Documento: "12345678000199",
			Nome:      "Empresa Teste Ltda",
			Municipio: "Sao Paulo",
		},
		Tomador: &TomadorInfo{
			Documento: &doc,
			Nome:      "Cliente Teste Ltda",
		},
		Servico: ServicoInfo{
			CodigoNacional: "010801",
			Descricao:      "Servicos de consultoria em tecnologia",
			LocalPrestacao: "Sao Paulo",
		},
		Valores: ValoresInfo{
			ValorServico: 1000.00,
			BaseCalculo:  1000.00,
			Aliquota:     &aliquota,
			ValorISSQN:   &valorISSQN,
			ValorLiquido: 950.00,
		},
		XML: "<NFSe>...</NFSe>",
	}

	// Serialize to JSON
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal NFSeQueryResponse: %v", err)
	}

	// Verify key fields are present with correct names
	jsonStr := string(data)
	expectedFields := []string{
		`"chave_acesso"`,
		`"numero"`,
		`"data_emissao"`,
		`"status"`,
		`"prestador"`,
		`"tomador"`,
		`"servico"`,
		`"valores"`,
		`"xml"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON missing expected field: %s", field)
		}
	}

	// Deserialize back
	var parsed NFSeQueryResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal NFSeQueryResponse: %v", err)
	}

	if parsed.ChaveAcesso != response.ChaveAcesso {
		t.Errorf("ChaveAcesso = %v, want %v", parsed.ChaveAcesso, response.ChaveAcesso)
	}
	if parsed.Status != response.Status {
		t.Errorf("Status = %v, want %v", parsed.Status, response.Status)
	}
}

func TestNFSeQueryResponse_OmitEmptyTomador(t *testing.T) {
	response := &NFSeQueryResponse{
		ChaveAcesso: "NFSe35503082024010112345678000199000010000000001",
		Numero:      "000001",
		DataEmissao: "2024-01-01T10:30:00Z",
		Status:      NFSeStatusActive,
		Tomador:     nil, // Anonymous service
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Tomador should be omitted when nil
	if contains(string(data), `"tomador"`) {
		t.Error("tomador field should be omitted when nil")
	}
}

func TestDPSLookupResponse_JSONSerialization(t *testing.T) {
	response := &DPSLookupResponse{
		DPSID:       "3550308112345678000199000010000000000000001",
		ChaveAcesso: "NFSe35503082024010112345678000199000010000000001",
		NFSeURL:     "/v1/nfse/NFSe35503082024010112345678000199000010000000001",
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal DPSLookupResponse: %v", err)
	}

	// Verify key fields
	jsonStr := string(data)
	expectedFields := []string{
		`"dps_id"`,
		`"chave_acesso"`,
		`"nfse_url"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON missing expected field: %s", field)
		}
	}

	// Deserialize back
	var parsed DPSLookupResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal DPSLookupResponse: %v", err)
	}

	if parsed.DPSID != response.DPSID {
		t.Errorf("DPSID = %v, want %v", parsed.DPSID, response.DPSID)
	}
}

func TestEventsQueryResponse_JSONSerialization(t *testing.T) {
	response := &EventsQueryResponse{
		ChaveAcesso: "NFSe35503082024010112345678000199000010000000001",
		Total:       2,
		Eventos: []EventInfo{
			{
				Tipo:      EventTypeEmission,
				Descricao: "NFS-e emitida",
				Sequencia: 1,
				Data:      "2024-01-01T10:30:00Z",
				XML:       "<evento>...</evento>",
			},
			{
				Tipo:      EventTypeCancellationCode,
				Descricao: "Cancelamento de NFS-e",
				Sequencia: 2,
				Data:      "2024-01-02T14:00:00Z",
				XML:       "<evento>...</evento>",
			},
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal EventsQueryResponse: %v", err)
	}

	// Verify key fields
	jsonStr := string(data)
	expectedFields := []string{
		`"chave_acesso"`,
		`"total"`,
		`"eventos"`,
		`"tipo"`,
		`"descricao"`,
		`"sequencia"`,
		`"data"`,
		`"xml"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON missing expected field: %s", field)
		}
	}

	// Deserialize back
	var parsed EventsQueryResponse
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal EventsQueryResponse: %v", err)
	}

	if parsed.Total != 2 {
		t.Errorf("Total = %v, want %v", parsed.Total, 2)
	}
	if len(parsed.Eventos) != 2 {
		t.Errorf("len(Eventos) = %v, want %v", len(parsed.Eventos), 2)
	}
}

func TestPrestadorInfo_JSONSerialization(t *testing.T) {
	prestador := PrestadorInfo{
		Documento: "12345678000199",
		Nome:      "Empresa Teste",
		Municipio: "Sao Paulo",
	}

	data, err := json.Marshal(prestador)
	if err != nil {
		t.Fatalf("Failed to marshal PrestadorInfo: %v", err)
	}

	jsonStr := string(data)
	expectedFields := []string{
		`"documento"`,
		`"nome"`,
		`"municipio"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON missing expected field: %s", field)
		}
	}
}

func TestValoresInfo_AllFields(t *testing.T) {
	aliquota := 5.00
	valorISSQN := 415.00

	valores := ValoresInfo{
		ValorServico: 10000.00,
		BaseCalculo:  8300.00,
		Aliquota:     &aliquota,
		ValorISSQN:   &valorISSQN,
		ValorLiquido: 8300.00,
	}

	data, err := json.Marshal(valores)
	if err != nil {
		t.Fatalf("Failed to marshal ValoresInfo: %v", err)
	}

	jsonStr := string(data)
	expectedFields := []string{
		`"valor_servico"`,
		`"base_calculo"`,
		`"aliquota"`,
		`"valor_issqn"`,
		`"valor_liquido"`,
	}

	for _, field := range expectedFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON missing expected field: %s", field)
		}
	}
}

func TestValoresInfo_OmitOptionalFields(t *testing.T) {
	valores := ValoresInfo{
		ValorServico: 1000.00,
		BaseCalculo:  1000.00,
		ValorLiquido: 1000.00,
		// Aliquota and ValorISSQN not set (nil)
	}

	data, err := json.Marshal(valores)
	if err != nil {
		t.Fatalf("Failed to marshal ValoresInfo: %v", err)
	}

	jsonStr := string(data)

	// Optional fields should be omitted when nil
	if contains(jsonStr, `"aliquota"`) {
		t.Error("aliquota should be omitted when nil")
	}
	if contains(jsonStr, `"valor_issqn"`) {
		t.Error("valor_issqn should be omitted when nil")
	}
}

func TestNewNFSeQueryResponse(t *testing.T) {
	response := NewNFSeQueryResponse(
		"NFSe35503082024010112345678000199000010000000001",
		"000001",
		"2024-01-01T10:30:00Z",
		NFSeStatusActive,
	)

	if response.ChaveAcesso != "NFSe35503082024010112345678000199000010000000001" {
		t.Errorf("ChaveAcesso = %v, want %v", response.ChaveAcesso, "NFSe35503082024010112345678000199000010000000001")
	}
	if response.Numero != "000001" {
		t.Errorf("Numero = %v, want %v", response.Numero, "000001")
	}
	if response.Status != NFSeStatusActive {
		t.Errorf("Status = %v, want %v", response.Status, NFSeStatusActive)
	}
}

func TestNewPrestadorInfo(t *testing.T) {
	prestador := NewPrestadorInfo("12345678000199", "Empresa Teste", "Sao Paulo")

	if prestador.Documento != "12345678000199" {
		t.Errorf("Documento = %v, want %v", prestador.Documento, "12345678000199")
	}
	if prestador.Nome != "Empresa Teste" {
		t.Errorf("Nome = %v, want %v", prestador.Nome, "Empresa Teste")
	}
	if prestador.Municipio != "Sao Paulo" {
		t.Errorf("Municipio = %v, want %v", prestador.Municipio, "Sao Paulo")
	}
}

func TestNewTomadorInfo(t *testing.T) {
	tomador := NewTomadorInfo("Cliente Teste")

	if tomador.Nome != "Cliente Teste" {
		t.Errorf("Nome = %v, want %v", tomador.Nome, "Cliente Teste")
	}
	if tomador.Documento != nil {
		t.Error("Documento should be nil initially")
	}

	// Test SetDocumento
	tomador.SetDocumento("98765432000188")
	if tomador.Documento == nil {
		t.Fatal("Documento should not be nil after SetDocumento")
	}
	if *tomador.Documento != "98765432000188" {
		t.Errorf("Documento = %v, want %v", *tomador.Documento, "98765432000188")
	}
}

func TestNewServicoInfo(t *testing.T) {
	servico := NewServicoInfo("010801", "Consultoria em TI", "Sao Paulo")

	if servico.CodigoNacional != "010801" {
		t.Errorf("CodigoNacional = %v, want %v", servico.CodigoNacional, "010801")
	}
	if servico.Descricao != "Consultoria em TI" {
		t.Errorf("Descricao = %v, want %v", servico.Descricao, "Consultoria em TI")
	}
	if servico.LocalPrestacao != "Sao Paulo" {
		t.Errorf("LocalPrestacao = %v, want %v", servico.LocalPrestacao, "Sao Paulo")
	}
}

func TestNewValoresInfo(t *testing.T) {
	valores := NewValoresInfo(1000.00, 1000.00, 950.00)

	if valores.ValorServico != 1000.00 {
		t.Errorf("ValorServico = %v, want %v", valores.ValorServico, 1000.00)
	}
	if valores.BaseCalculo != 1000.00 {
		t.Errorf("BaseCalculo = %v, want %v", valores.BaseCalculo, 1000.00)
	}
	if valores.ValorLiquido != 950.00 {
		t.Errorf("ValorLiquido = %v, want %v", valores.ValorLiquido, 950.00)
	}

	// Test SetAliquota
	valores.SetAliquota(5.00)
	if valores.Aliquota == nil {
		t.Fatal("Aliquota should not be nil after SetAliquota")
	}
	if *valores.Aliquota != 5.00 {
		t.Errorf("Aliquota = %v, want %v", *valores.Aliquota, 5.00)
	}

	// Test SetValorISSQN
	valores.SetValorISSQN(50.00)
	if valores.ValorISSQN == nil {
		t.Fatal("ValorISSQN should not be nil after SetValorISSQN")
	}
	if *valores.ValorISSQN != 50.00 {
		t.Errorf("ValorISSQN = %v, want %v", *valores.ValorISSQN, 50.00)
	}
}

func TestNewDPSLookupResponse(t *testing.T) {
	response := NewDPSLookupResponse(
		"3550308112345678000199000010000000000000001",
		"NFSe35503082024010112345678000199000010000000001",
		"/v1/nfse/NFSe35503082024010112345678000199000010000000001",
	)

	if response.DPSID != "3550308112345678000199000010000000000000001" {
		t.Errorf("DPSID = %v, want %v", response.DPSID, "3550308112345678000199000010000000000000001")
	}
	if response.ChaveAcesso != "NFSe35503082024010112345678000199000010000000001" {
		t.Errorf("ChaveAcesso = %v, want %v", response.ChaveAcesso, "NFSe35503082024010112345678000199000010000000001")
	}
}

func TestNewEventsQueryResponse(t *testing.T) {
	eventos := []EventInfo{
		{Tipo: EventTypeEmission, Sequencia: 1, Data: "2024-01-01T10:00:00Z", XML: "<evento/>"},
		{Tipo: EventTypeCancellationCode, Sequencia: 2, Data: "2024-01-02T14:00:00Z", XML: "<evento/>"},
	}

	response := NewEventsQueryResponse(
		"NFSe35503082024010112345678000199000010000000001",
		eventos,
	)

	if response.Total != 2 {
		t.Errorf("Total = %v, want %v", response.Total, 2)
	}
	if len(response.Eventos) != 2 {
		t.Errorf("len(Eventos) = %v, want %v", len(response.Eventos), 2)
	}
}

func TestNewEventInfo(t *testing.T) {
	tests := []struct {
		tipo          string
		wantDescricao string
	}{
		{EventTypeEmission, "NFS-e emitida"},
		{EventTypeCancellation, "NFS-e cancelada"},
		{EventTypeCancellationCode, "Cancelamento de NFS-e"},
		{EventTypeSubstitution, "NFS-e substituida"},
		{"UNKNOWN", "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.tipo, func(t *testing.T) {
			event := NewEventInfo(tt.tipo, 1, "2024-01-01T10:00:00Z", "<evento/>")
			if event.Tipo != tt.tipo {
				t.Errorf("Tipo = %v, want %v", event.Tipo, tt.tipo)
			}
			if event.Descricao != tt.wantDescricao {
				t.Errorf("Descricao = %v, want %v", event.Descricao, tt.wantDescricao)
			}
			if event.Sequencia != 1 {
				t.Errorf("Sequencia = %v, want %v", event.Sequencia, 1)
			}
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	// Fixed time for testing
	tm := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)

	formatted := FormatDateTime(tm)
	expected := "2024-01-15T10:30:45Z"

	if formatted != expected {
		t.Errorf("FormatDateTime() = %v, want %v", formatted, expected)
	}
}

func TestFormatDate(t *testing.T) {
	tm := time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC)

	formatted := FormatDate(tm)
	expected := "2024-01-15"

	if formatted != expected {
		t.Errorf("FormatDate() = %v, want %v", formatted, expected)
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify status constants have expected values
	if NFSeStatusActive != "active" {
		t.Errorf("NFSeStatusActive = %v, want %v", NFSeStatusActive, "active")
	}
	if NFSeStatusCancelled != "cancelled" {
		t.Errorf("NFSeStatusCancelled = %v, want %v", NFSeStatusCancelled, "cancelled")
	}
	if NFSeStatusSubstituted != "substituted" {
		t.Errorf("NFSeStatusSubstituted = %v, want %v", NFSeStatusSubstituted, "substituted")
	}
}

func TestEventTypeDescriptions(t *testing.T) {
	expectedDescriptions := map[string]string{
		EventTypeEmission:         "NFS-e emitida",
		EventTypeCancellation:     "NFS-e cancelada",
		EventTypeCancellationCode: "Cancelamento de NFS-e",
		EventTypeSubstitution:     "NFS-e substituida",
		EventTypeCorrection:       "Carta de correcao registrada",
		EventTypeLockout:          "NFS-e bloqueada pelo fisco",
	}

	for eventType, expectedDesc := range expectedDescriptions {
		if desc, ok := EventTypeDescriptions[eventType]; !ok {
			t.Errorf("EventTypeDescriptions missing key: %s", eventType)
		} else if desc != expectedDesc {
			t.Errorf("EventTypeDescriptions[%s] = %v, want %v", eventType, desc, expectedDesc)
		}
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
