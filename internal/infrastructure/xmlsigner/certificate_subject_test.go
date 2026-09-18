package xmlsigner

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"testing"
)

// ICP-Brasil writes the holder as "RAZAO SOCIAL:CNPJ". Everything else in the
// table is a common name that merely looks like one.
func TestSubjectCNPJ(t *testing.T) {
	tests := []struct {
		name     string
		cn       string
		wantCNPJ string
		wantNome string
	}{
		{
			name:     "e-CNPJ",
			cn:       "EMPRESA TESTE LTDA:12345678000195",
			wantCNPJ: "12345678000195",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name:     "digitos verificadores errados",
			cn:       "EMPRESA TESTE LTDA:12345678000100",
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA:12345678000100",
		},
		{
			name:     "e-CPF",
			cn:       "FULANO DE TAL:12345678909",
			wantCNPJ: "",
			wantNome: "FULANO DE TAL:12345678909",
		},
		{
			name:     "sem sufixo",
			cn:       "EMPRESA TESTE LTDA",
			wantCNPJ: "",
			wantNome: "EMPRESA TESTE LTDA",
		},
		{
			name:     "nome com dois pontos",
			cn:       "EMPRESA: A MELHOR:12345678000195",
			wantCNPJ: "12345678000195",
			wantNome: "EMPRESA: A MELHOR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &CertificateInfo{
				Certificate: &x509.Certificate{Subject: pkix.Name{CommonName: tt.cn}},
			}

			if got := info.SubjectCNPJ(); got != tt.wantCNPJ {
				t.Errorf("SubjectCNPJ() = %q, esperava %q", got, tt.wantCNPJ)
			}
			if got := info.SubjectHolderName(); got != tt.wantNome {
				t.Errorf("SubjectHolderName() = %q, esperava %q", got, tt.wantNome)
			}
		})
	}
}

func TestSubjectCNPJSemCertificado(t *testing.T) {
	var info CertificateInfo

	if got := info.SubjectCNPJ(); got != "" {
		t.Errorf("SubjectCNPJ() = %q, esperava vazio", got)
	}
	if got := info.SubjectHolderName(); got != "" {
		t.Errorf("SubjectHolderName() = %q, esperava vazio", got)
	}
}
