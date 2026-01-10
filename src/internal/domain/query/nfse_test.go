package query

import (
	"strings"
	"testing"
	"time"
)

func TestParseNFSeXML(t *testing.T) {
	tests := []struct {
		name        string
		xml         string
		wantErr     bool
		checkResult func(*testing.T, *NFSeData)
	}{
		{
			name: "valid complete NFS-e XML",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00">
  <infNFSe Id="NFSe3550308202601081234567890123456789012345678">
    <nNFSe>000000123</nNFSe>
    <dhEmi>2026-01-08T10:30:00-03:00</dhEmi>
    <chNFSe>NFSe3550308202601081123456789012300000000000012310</chNFSe>
    <sit>1</sit>
    <emit>
      <CNPJ>12345678000199</CNPJ>
      <xNome>Empresa Prestadora Ltda</xNome>
      <ender>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </ender>
    </emit>
    <toma>
      <CNPJ>98765432000188</CNPJ>
      <xNome>Cliente Tomador S.A.</xNome>
    </toma>
    <serv>
      <cTribNac>010201</cTribNac>
      <xDescServ>Servicos de consultoria em tecnologia da informacao</xDescServ>
      <localPrest>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </localPrest>
    </serv>
    <valores>
      <vServico>1000.00</vServico>
      <vBC>1000.00</vBC>
      <pAliq>5.00</pAliq>
      <vISS>50.00</vISS>
      <vLiq>950.00</vLiq>
    </valores>
  </infNFSe>
</NFSe>`,
			wantErr: false,
			checkResult: func(t *testing.T, data *NFSeData) {
				if data.ChaveAcesso != "NFSe3550308202601081123456789012300000000000012310" {
					t.Errorf("ChaveAcesso = %q, want NFSe3550308202601081123456789012300000000000012310", data.ChaveAcesso)
				}
				if data.Numero != "000000123" {
					t.Errorf("Numero = %q, want 000000123", data.Numero)
				}
				if data.Status != NFSeStatusActive {
					t.Errorf("Status = %q, want %q", data.Status, NFSeStatusActive)
				}
				if data.Prestador.Documento != "12345678000199" {
					t.Errorf("Prestador.Documento = %q, want 12345678000199", data.Prestador.Documento)
				}
				if data.Prestador.Nome != "Empresa Prestadora Ltda" {
					t.Errorf("Prestador.Nome = %q, want Empresa Prestadora Ltda", data.Prestador.Nome)
				}
				if data.Prestador.Municipio != "Sao Paulo" {
					t.Errorf("Prestador.Municipio = %q, want Sao Paulo", data.Prestador.Municipio)
				}
				if data.Tomador == nil {
					t.Error("Tomador should not be nil")
				} else {
					if data.Tomador.Documento != "98765432000188" {
						t.Errorf("Tomador.Documento = %q, want 98765432000188", data.Tomador.Documento)
					}
					if data.Tomador.TipoDocumento != "cnpj" {
						t.Errorf("Tomador.TipoDocumento = %q, want cnpj", data.Tomador.TipoDocumento)
					}
				}
				if data.Servico.CodigoNacional != "010201" {
					t.Errorf("Servico.CodigoNacional = %q, want 010201", data.Servico.CodigoNacional)
				}
				if data.Valores.ValorServico != 1000.00 {
					t.Errorf("Valores.ValorServico = %f, want 1000.00", data.Valores.ValorServico)
				}
				if data.Valores.ValorISSQN != 50.00 {
					t.Errorf("Valores.ValorISSQN = %f, want 50.00", data.Valores.ValorISSQN)
				}
			},
		},
		{
			name: "NFS-e with CPF taker",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infNFSe>
    <nNFSe>000000456</nNFSe>
    <dhEmi>2026-01-08T14:00:00-03:00</dhEmi>
    <chNFSe>NFSe3550308202601081123456789012300000000000045600</chNFSe>
    <sit>1</sit>
    <emit>
      <CNPJ>12345678000199</CNPJ>
      <xNome>Empresa Teste</xNome>
      <ender>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </ender>
    </emit>
    <toma>
      <CPF>12345678901</CPF>
      <xNome>Pessoa Fisica</xNome>
    </toma>
    <serv>
      <cTribNac>010101</cTribNac>
      <xDescServ>Servico para pessoa fisica</xDescServ>
      <localPrest>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </localPrest>
    </serv>
    <valores>
      <vServico>500.00</vServico>
      <vBC>500.00</vBC>
      <pAliq>5.00</pAliq>
      <vISS>25.00</vISS>
      <vLiq>475.00</vLiq>
    </valores>
  </infNFSe>
</NFSe>`,
			wantErr: false,
			checkResult: func(t *testing.T, data *NFSeData) {
				if data.Tomador == nil {
					t.Error("Tomador should not be nil")
					return
				}
				if data.Tomador.Documento != "12345678901" {
					t.Errorf("Tomador.Documento = %q, want 12345678901", data.Tomador.Documento)
				}
				if data.Tomador.TipoDocumento != "cpf" {
					t.Errorf("Tomador.TipoDocumento = %q, want cpf", data.Tomador.TipoDocumento)
				}
			},
		},
		{
			name: "NFS-e without taker (anonymous)",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infNFSe>
    <nNFSe>000000789</nNFSe>
    <dhEmi>2026-01-08T16:00:00-03:00</dhEmi>
    <chNFSe>NFSe3550308202601081123456789012300000000000078900</chNFSe>
    <sit>1</sit>
    <emit>
      <CNPJ>12345678000199</CNPJ>
      <xNome>Empresa Teste</xNome>
      <ender>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </ender>
    </emit>
    <serv>
      <cTribNac>010101</cTribNac>
      <xDescServ>Servico anonimo</xDescServ>
      <localPrest>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </localPrest>
    </serv>
    <valores>
      <vServico>100.00</vServico>
      <vBC>100.00</vBC>
      <pAliq>5.00</pAliq>
      <vISS>5.00</vISS>
      <vLiq>95.00</vLiq>
    </valores>
  </infNFSe>
</NFSe>`,
			wantErr: false,
			checkResult: func(t *testing.T, data *NFSeData) {
				if data.Tomador != nil {
					t.Error("Tomador should be nil for anonymous service")
				}
			},
		},
		{
			name: "cancelled NFS-e",
			xml: `<?xml version="1.0" encoding="UTF-8"?>
<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infNFSe>
    <nNFSe>000000999</nNFSe>
    <dhEmi>2026-01-07T10:00:00-03:00</dhEmi>
    <chNFSe>NFSe3550308202601071123456789012300000000000099900</chNFSe>
    <sit>2</sit>
    <emit>
      <CNPJ>12345678000199</CNPJ>
      <xNome>Empresa Teste</xNome>
      <ender>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </ender>
    </emit>
    <serv>
      <cTribNac>010101</cTribNac>
      <xDescServ>Servico cancelado</xDescServ>
      <localPrest>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </localPrest>
    </serv>
    <valores>
      <vServico>200.00</vServico>
      <vBC>200.00</vBC>
      <pAliq>5.00</pAliq>
      <vISS>10.00</vISS>
      <vLiq>190.00</vLiq>
    </valores>
  </infNFSe>
</NFSe>`,
			wantErr: false,
			checkResult: func(t *testing.T, data *NFSeData) {
				if data.Status != NFSeStatusCancelled {
					t.Errorf("Status = %q, want %q", data.Status, NFSeStatusCancelled)
				}
			},
		},
		{
			name:    "empty XML",
			xml:     "",
			wantErr: true,
		},
		{
			name:    "invalid XML",
			xml:     "not valid xml <",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ParseNFSeXML(tt.xml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseNFSeXML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, data)
			}
		})
	}
}

func TestNFSeData_ToQueryResponse(t *testing.T) {
	data := &NFSeData{
		ChaveAcesso: "NFSe3550308202601081123456789012300000000000012310",
		Numero:      "000000123",
		DataEmissao: time.Date(2026, 1, 8, 10, 30, 0, 0, time.FixedZone("BRT", -3*3600)),
		Status:      NFSeStatusActive,
		Prestador: PrestadorData{
			Documento:       "12345678000199",
			Nome:            "Empresa Teste",
			Municipio:       "Sao Paulo",
			MunicipioCodigo: "3550308",
		},
		Tomador: &TomadorData{
			Documento:     "98765432000188",
			TipoDocumento: "cnpj",
			Nome:          "Cliente Teste",
		},
		Servico: ServicoData{
			CodigoNacional:  "010201",
			Descricao:       "Consultoria em TI",
			LocalPrestacao:  "Sao Paulo - SP",
			MunicipioCodigo: "3550308",
		},
		Valores: ValoresData{
			ValorServico: 1000.00,
			BaseCalculo:  1000.00,
			Aliquota:     5.00,
			ValorISSQN:   50.00,
			ValorLiquido: 950.00,
		},
	}

	originalXML := "<NFSe>test</NFSe>"
	response := data.ToQueryResponse(originalXML)

	if response.ChaveAcesso != data.ChaveAcesso {
		t.Errorf("ChaveAcesso = %q, want %q", response.ChaveAcesso, data.ChaveAcesso)
	}
	if response.Numero != data.Numero {
		t.Errorf("Numero = %q, want %q", response.Numero, data.Numero)
	}
	if response.Status != data.Status {
		t.Errorf("Status = %q, want %q", response.Status, data.Status)
	}
	if response.XML != originalXML {
		t.Errorf("XML = %q, want %q", response.XML, originalXML)
	}
	if response.Prestador.Documento != data.Prestador.Documento {
		t.Errorf("Prestador.Documento = %q, want %q", response.Prestador.Documento, data.Prestador.Documento)
	}
	if response.Tomador == nil {
		t.Error("Tomador should not be nil")
	} else {
		if response.Tomador.Nome != data.Tomador.Nome {
			t.Errorf("Tomador.Nome = %q, want %q", response.Tomador.Nome, data.Tomador.Nome)
		}
	}
	if response.Valores.ValorServico != data.Valores.ValorServico {
		t.Errorf("Valores.ValorServico = %f, want %f", response.Valores.ValorServico, data.Valores.ValorServico)
	}
	if response.Valores.Aliquota == nil || *response.Valores.Aliquota != data.Valores.Aliquota {
		t.Errorf("Valores.Aliquota should be set to %f", data.Valores.Aliquota)
	}
}

func TestParseDateTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"RFC3339", "2026-01-08T10:30:00-03:00", false},
		{"RFC3339 with Z", "2026-01-08T10:30:00Z", false},
		{"date only", "2026-01-08", false},
		{"empty", "", true},
		{"invalid", "not a date", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseDateTime(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDateTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestMapStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1", NFSeStatusActive},
		{"100", NFSeStatusActive},
		{"2", NFSeStatusCancelled},
		{"101", NFSeStatusCancelled},
		{"3", NFSeStatusSubstituted},
		{"102", NFSeStatusSubstituted},
		{"999", "999"},
		{"", NFSeStatusActive},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapStatus(tt.input)
			if got != tt.want {
				t.Errorf("mapStatus(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatLocalPrestacao(t *testing.T) {
	tests := []struct {
		name string
		loc  localPrestXML
		want string
	}{
		{
			name: "complete location",
			loc:  localPrestXML{XMun: "Sao Paulo", UF: "SP"},
			want: "Sao Paulo - SP",
		},
		{
			name: "only municipality",
			loc:  localPrestXML{XMun: "Sao Paulo"},
			want: "Sao Paulo",
		},
		{
			name: "empty",
			loc:  localPrestXML{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatLocalPrestacao(tt.loc)
			if got != tt.want {
				t.Errorf("formatLocalPrestacao() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseNFSeXML_WithBOM(t *testing.T) {
	// XML with BOM prefix
	xmlWithBOM := "\xef\xbb\xbf" + `<?xml version="1.0" encoding="UTF-8"?>
<NFSe xmlns="http://www.sped.fazenda.gov.br/nfse">
  <infNFSe>
    <nNFSe>000000001</nNFSe>
    <dhEmi>2026-01-08T10:00:00-03:00</dhEmi>
    <chNFSe>NFSe3550308202601081123456789012300000000000000100</chNFSe>
    <sit>1</sit>
    <emit>
      <CNPJ>12345678000199</CNPJ>
      <xNome>Test</xNome>
      <ender>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </ender>
    </emit>
    <serv>
      <cTribNac>010101</cTribNac>
      <xDescServ>Test</xDescServ>
      <localPrest>
        <cMun>3550308</cMun>
        <xMun>Sao Paulo</xMun>
        <UF>SP</UF>
      </localPrest>
    </serv>
    <valores>
      <vServico>100.00</vServico>
      <vBC>100.00</vBC>
      <pAliq>5.00</pAliq>
      <vISS>5.00</vISS>
      <vLiq>95.00</vLiq>
    </valores>
  </infNFSe>
</NFSe>`

	data, err := ParseNFSeXML(xmlWithBOM)
	if err != nil {
		t.Errorf("ParseNFSeXML with BOM failed: %v", err)
		return
	}
	if data.Numero != "000000001" {
		t.Errorf("Numero = %q, want 000000001", data.Numero)
	}
}

func TestFormatDateTimeISO(t *testing.T) {
	// Test zero time
	zeroTime := time.Time{}
	if result := formatDateTimeISO(zeroTime); result != "" {
		t.Errorf("formatDateTimeISO(zero) = %q, want empty string", result)
	}

	// Test valid time
	validTime := time.Date(2026, 1, 8, 10, 30, 0, 0, time.UTC)
	result := formatDateTimeISO(validTime)
	if !strings.Contains(result, "2026-01-08") {
		t.Errorf("formatDateTimeISO() = %q, should contain date", result)
	}
}
