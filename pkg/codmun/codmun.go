// Package codmun checks the seven-digit IBGE municipality codes that the
// DPS carries in cLocEmi, cLocPrestacao and every address.
//
// It holds no list of municipalities: the code carries its own state prefix
// and check digit, which is enough to catch a mistyped or misread code without
// shipping the IBGE registry inside the binary (ADR 0012).
package codmun

import "fmt"

// Tamanho is the length of an IBGE municipality code.
const Tamanho = 7

// ufPorPrefixo maps the first two digits of a municipality code — the state's
// own IBGE code — to its abbreviation.
var ufPorPrefixo = map[string]string{
	"11": "RO", "12": "AC", "13": "AM", "14": "RR", "15": "PA", "16": "AP", "17": "TO",
	"21": "MA", "22": "PI", "23": "CE", "24": "RN", "25": "PB", "26": "PE", "27": "AL",
	"28": "SE", "29": "BA",
	"31": "MG", "32": "ES", "33": "RJ", "35": "SP",
	"41": "PR", "42": "SC", "43": "RS",
	"50": "MS", "51": "MT", "52": "GO", "53": "DF",
}

// semDigitoVerificador lists the codes the IBGE issued without a valid check
// digit. They are real municipalities, and the official table (ANEXO_A of the
// Sistema Nacional) has exactly these nine; a test holds the algorithm to it.
var semDigitoVerificador = map[string]bool{
	"2201919": true, // Bom Princípio do Piauí/PI
	"2201988": true, // Brejo do Piauí/PI
	"2202251": true, // Canavieira/PI
	"2611533": true, // Quixaba/PE
	"3117836": true, // Cônego Marinho/MG
	"3152131": true, // Ponto Chique/MG
	"4305871": true, // Coronel Barros/RS
	"5203939": true, // Buriti de Goiás/GO
	"5203962": true, // Buritinópolis/GO
}

// UF returns the state abbreviation of a municipality code, or "" when the
// code is not seven digits or its prefix is not a state.
func UF(codigo string) string {
	if !digitos(codigo) {
		return ""
	}
	return ufPorPrefixo[codigo[:2]]
}

// UFExiste reports whether sigla is the abbreviation of one of the 27
// federative units, in upper case.
func UFExiste(sigla string) bool {
	for _, uf := range ufPorPrefixo {
		if uf == sigla {
			return true
		}
	}
	return false
}

// Validar reports why codigo cannot be an IBGE municipality code, or nil.
//
// A code that passes may still be the wrong municipality — only a lookup can
// tell Curitiba from its neighbour — but a transposed or mistyped digit, a
// four-digit TOM code from the Receita Federal, or a state code do not pass.
func Validar(codigo string) error {
	if !digitos(codigo) {
		return fmt.Errorf("o codigo do municipio tem %d digitos numericos; %q nao tem esse formato", Tamanho, codigo)
	}
	if UF(codigo) == "" {
		return fmt.Errorf("%q nao comeca com o codigo de uma UF", codigo)
	}
	if digitoVerificador(codigo) != codigo[6] && !semDigitoVerificador[codigo] {
		return fmt.Errorf("o digito verificador de %q nao confere; confira o codigo na tabela do IBGE", codigo)
	}
	return nil
}

// digitoVerificador computes the IBGE check digit: the first six digits are
// weighted 1,2,1,2,1,2, the digits of each product are summed, and the check
// digit completes the sum to a multiple of ten.
func digitoVerificador(codigo string) byte {
	soma := 0
	for i := 0; i < 6; i++ {
		p := int(codigo[i]-'0') * (1 + i%2)
		soma += p/10 + p%10
	}
	return byte('0' + (10-soma%10)%10)
}

func digitos(codigo string) bool {
	if len(codigo) != Tamanho {
		return false
	}
	for i := 0; i < len(codigo); i++ {
		if codigo[i] < '0' || codigo[i] > '9' {
			return false
		}
	}
	return true
}
