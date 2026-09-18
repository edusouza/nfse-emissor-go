package xmlbuilder

import (
	"strings"
	"testing"
)

// The two codes are the reverse of what their names suggest, and the reversal
// is the whole point of this test. From the E0004 rule in
// docs/anexos/ANEXO_I-SEFIN_ADN-DPS_NFSe-SNNFSe-v1.00-20251226.xlsx:
//
//	Tipo de inscrição Federal = 1 / Inscrição Federal = CPF emitente da DPS;
//	Tipo de inscrição Federal = 2 / Inscrição Federal = CNPJ emitente da DPS;
//
// They were swapped, and the government answered E0004 on every emission a
// company made — which is every emission this tool exists for.
func TestRegistrationTypeCodesMatchTheOfficialRule(t *testing.T) {
	if RegistrationTypeCPF != 1 {
		t.Errorf("RegistrationTypeCPF = %d, a regra E0004 diz 1", RegistrationTypeCPF)
	}
	if RegistrationTypeCNPJ != 2 {
		t.Errorf("RegistrationTypeCNPJ = %d, a regra E0004 diz 2", RegistrationTypeCNPJ)
	}
}

// The identifier is the concatenation the government re-derives from the DPS
// fields, so each slice of it has to land where the rule says.
func TestGenerateDPSID_LayoutForCNPJ(t *testing.T) {
	id, err := GenerateDPSID(DPSIDConfig{
		MunicipalityCode:    "4106902",
		RegistrationType:    RegistrationTypeCNPJ,
		FederalRegistration: "12345678000195",
		Series:              "00001",
		Number:              "42",
	})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(id, "DPS") {
		t.Fatalf("identificador sem o literal DPS: %q", id)
	}
	digits := strings.TrimPrefix(id, "DPS")
	if len(digits) != 42 {
		t.Fatalf("identificador com %d digitos, esperava 42: %q", len(digits), digits)
	}

	for _, tc := range []struct {
		campo string
		got   string
		want  string
	}{
		{"codigo do municipio", digits[0:7], "4106902"},
		{"tipo de inscricao", digits[7:8], "2"},
		{"inscricao federal", digits[8:22], "12345678000195"},
		{"serie", digits[22:27], "00001"},
		{"numero", digits[27:42], "000000000000042"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, esperava %q", tc.campo, tc.got, tc.want)
		}
	}
}

func TestGenerateDPSID_LayoutForCPF(t *testing.T) {
	id, err := GenerateDPSID(DPSIDConfig{
		MunicipalityCode:    "4106902",
		RegistrationType:    RegistrationTypeCPF,
		FederalRegistration: "12345678901",
		Series:              "00001",
		Number:              "1",
	})
	if err != nil {
		t.Fatal(err)
	}

	digits := strings.TrimPrefix(id, "DPS")
	if digits[7:8] != "1" {
		t.Errorf("tipo de inscricao = %q, esperava 1 para CPF", digits[7:8])
	}
	// Eleven digits in a field of fourteen: the rule says to pad on the left.
	if digits[8:22] != "00012345678901" {
		t.Errorf("inscricao federal = %q, esperava o CPF com zeros a esquerda", digits[8:22])
	}
}

// A length that belongs to the other registration type must be refused rather
// than padded or truncated into something the government will reject later.
func TestGenerateDPSID_RefusesMismatchedLength(t *testing.T) {
	if _, err := GenerateDPSID(DPSIDConfig{
		MunicipalityCode:    "4106902",
		RegistrationType:    RegistrationTypeCNPJ,
		FederalRegistration: "12345678901", // um CPF
		Series:              "00001",
		Number:              "1",
	}); err == nil {
		t.Error("esperava recusa de um CPF declarado como CNPJ")
	}

	if _, err := GenerateDPSID(DPSIDConfig{
		MunicipalityCode:    "4106902",
		RegistrationType:    7,
		FederalRegistration: "12345678000195",
		Series:              "00001",
		Number:              "1",
	}); err == nil {
		t.Error("esperava recusa de um tipo de inscricao invalido")
	}
}
