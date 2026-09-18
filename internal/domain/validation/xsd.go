// Package validation provides validation logic for NFS-e domain objects.
package validation

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// XSD validation error types.
const (
	XSDErrorMissingElement     = "MISSING_ELEMENT"
	XSDErrorInvalidNamespace   = "INVALID_NAMESPACE"
	XSDErrorInvalidValue       = "INVALID_VALUE"
	XSDErrorInvalidFormat      = "INVALID_FORMAT"
	XSDErrorInvalidDataType    = "INVALID_DATA_TYPE"
	XSDErrorMissingAttribute   = "MISSING_ATTRIBUTE"
	XSDErrorUnexpectedElement  = "UNEXPECTED_ELEMENT"
	XSDErrorInvalidEnvironment = "INVALID_ENVIRONMENT"
)

// XSDValidationError represents a single XSD validation error.
type XSDValidationError struct {
	// Code is a machine-readable error code.
	Code string `json:"code"`

	// Element is the XPath or name of the element that failed validation.
	Element string `json:"element"`

	// Message is a human-readable error message.
	Message string `json:"message"`

	// Value is the actual value that failed validation (if applicable).
	Value string `json:"value,omitempty"`
}

// Error implements the error interface.
func (e XSDValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("%s [%s]: %s (value: %s)", e.Code, e.Element, e.Message, e.Value)
	}
	return fmt.Sprintf("%s [%s]: %s", e.Code, e.Element, e.Message)
}

// Expected namespace for NFS-e documents.
const (
	NFSeNamespace = "http://www.sped.fazenda.gov.br/nfse"
)

// XSDValidator validates DPS XML documents against the NFS-e schema.
// Note: This is a structural validator that checks required elements,
// data types, and formats. It does not perform full XSD schema validation
// which would require an external library.
type XSDValidator struct {
	// SchemaDir is the directory containing XSD schema files.
	// Currently not used as we implement structural validation.
	SchemaDir string
}

// NewXSDValidator creates a new XSD validator.
//
// Parameters:
//   - schemaDir: The directory containing XSD schema files (for future use)
//
// Returns:
//   - *XSDValidator: A new validator instance
//   - error: Always nil in current implementation
func NewXSDValidator(schemaDir string) (*XSDValidator, error) {
	return &XSDValidator{
		SchemaDir: schemaDir,
	}, nil
}

// ValidateDPS validates a DPS XML document against the NFS-e schema.
// This performs structural validation checking:
//   - Root element is DPS with correct namespace
//   - Required elements exist: infDPS, tpAmb, dhEmi, prest, serv, valores
//   - Element data types are correct (dates, numbers, strings)
//
// Parameters:
//   - dpsXML: The DPS XML document as a string
//
// Returns:
//   - []XSDValidationError: A slice of validation errors (empty if valid)
func (v *XSDValidator) ValidateDPS(dpsXML string) []XSDValidationError {
	var errors []XSDValidationError

	// Parse the XML document
	doc := etree.NewDocument()
	if err := doc.ReadFromString(dpsXML); err != nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidFormat,
			Element: "document",
			Message: fmt.Sprintf("failed to parse XML: %v", err),
		})
		return errors
	}

	// Find the root DPS element
	dps := doc.Root()
	if dps == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "DPS",
			Message: "root DPS element not found",
		})
		return errors
	}

	// Validate root element name
	if dps.Tag != "DPS" {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "DPS",
			Message: fmt.Sprintf("expected root element 'DPS', found '%s'", dps.Tag),
			Value:   dps.Tag,
		})
		return errors
	}

	// Validate namespace (check if NFS-e namespace is present)
	errors = append(errors, v.validateNamespace(dps)...)

	// Find infDPS element
	infDPS := dps.FindElement("infDPS")
	if infDPS == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "DPS/infDPS",
			Message: "required element 'infDPS' not found",
		})
		return errors
	}

	// Validate infDPS has Id attribute
	if infDPS.SelectAttr("Id") == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingAttribute,
			Element: "DPS/infDPS",
			Message: "required attribute 'Id' not found on infDPS",
		})
	}

	// Validate required child elements of infDPS
	errors = append(errors, v.validateInfDPS(infDPS)...)

	return errors
}

// validateNamespace checks if the DPS element has the correct namespace.
func (v *XSDValidator) validateNamespace(dps *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	// Check element's namespace
	hasValidNS := false

	// Check if the element itself has the NFS-e namespace
	if dps.Space == NFSeNamespace {
		hasValidNS = true
	}

	// Check xmlns attributes
	for _, attr := range dps.Attr {
		if attr.Key == "xmlns" && attr.Value == NFSeNamespace {
			hasValidNS = true
			break
		}
		if attr.Space == "xmlns" && attr.Value == NFSeNamespace {
			hasValidNS = true
			break
		}
	}

	if !hasValidNS {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidNamespace,
			Element: "DPS",
			Message: fmt.Sprintf("DPS element should have namespace '%s'", NFSeNamespace),
		})
	}

	return errors
}

// validateInfDPS validates the infDPS element and its required children.
func (v *XSDValidator) validateInfDPS(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	// Validate tpAmb (environment type) - required
	errors = append(errors, v.validateTpAmb(infDPS)...)

	// Validate dhEmi (emission date/time) - required
	errors = append(errors, v.validateDhEmi(infDPS)...)

	// Validate verAplic (application version) - required
	errors = append(errors, v.validateRequiredElement(infDPS, "verAplic", "application version")...)

	// Validate serie (series) - required
	errors = append(errors, v.validateSerie(infDPS)...)

	// Validate nDPS (DPS number) - required
	errors = append(errors, v.validateNDPS(infDPS)...)

	// Validate dCompet (competence date) - required
	errors = append(errors, v.validateDCompet(infDPS)...)

	// Validate tpEmit (emitter type) - required
	errors = append(errors, v.validateTpEmit(infDPS)...)

	// Validate cLocEmi (emission municipality code) - required
	errors = append(errors, v.validateCLocEmi(infDPS)...)

	// Validate subst (substitution) - required
	errors = append(errors, v.validateSubst(infDPS)...)

	// Validate prest (provider) - required
	errors = append(errors, v.validatePrest(infDPS)...)

	// Validate serv (service) - required
	errors = append(errors, v.validateServ(infDPS)...)

	// Validate valores (values) - required
	errors = append(errors, v.validateValores(infDPS)...)

	return errors
}

// validateTpAmb validates the tpAmb (environment type) element.
func (v *XSDValidator) validateTpAmb(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	tpAmb := infDPS.FindElement("tpAmb")
	if tpAmb == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/tpAmb",
			Message: "required element 'tpAmb' (environment type) not found",
		})
		return errors
	}

	value := strings.TrimSpace(tpAmb.Text())
	if value != "1" && value != "2" {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidValue,
			Element: "infDPS/tpAmb",
			Message: "tpAmb must be '1' (production) or '2' (homologation)",
			Value:   value,
		})
	}

	return errors
}

// validateDhEmi validates the dhEmi (emission date/time) element.
func (v *XSDValidator) validateDhEmi(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	dhEmi := infDPS.FindElement("dhEmi")
	if dhEmi == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/dhEmi",
			Message: "required element 'dhEmi' (emission date/time) not found",
		})
		return errors
	}

	value := strings.TrimSpace(dhEmi.Text())
	if !isValidXSDDateTime(value) {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidDataType,
			Element: "infDPS/dhEmi",
			Message: "dhEmi must be a valid ISO 8601 datetime (e.g., 2024-01-15T10:30:00-03:00)",
			Value:   value,
		})
	}

	return errors
}

// validateSerie validates the serie element.
func (v *XSDValidator) validateSerie(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	serie := infDPS.FindElement("serie")
	if serie == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serie",
			Message: "required element 'serie' not found",
		})
		return errors
	}

	value := strings.TrimSpace(serie.Text())
	// Series should be 5 digits
	if !regexp.MustCompile(`^\d{5}$`).MatchString(value) {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidFormat,
			Element: "infDPS/serie",
			Message: "serie must be exactly 5 digits",
			Value:   value,
		})
	}

	return errors
}

// validateNDPS validates the nDPS (DPS number) element.
func (v *XSDValidator) validateNDPS(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	nDPS := infDPS.FindElement("nDPS")
	if nDPS == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/nDPS",
			Message: "required element 'nDPS' (DPS number) not found",
		})
		return errors
	}

	value := strings.TrimSpace(nDPS.Text())
	// DPS number should be 1-15 digits
	if !regexp.MustCompile(`^\d{1,15}$`).MatchString(value) {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidFormat,
			Element: "infDPS/nDPS",
			Message: "nDPS must be 1 to 15 digits",
			Value:   value,
		})
	}

	return errors
}

// validateDCompet validates the dCompet (competence date) element.
func (v *XSDValidator) validateDCompet(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	dCompet := infDPS.FindElement("dCompet")
	if dCompet == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/dCompet",
			Message: "required element 'dCompet' (competence date) not found",
		})
		return errors
	}

	value := strings.TrimSpace(dCompet.Text())
	if !isValidXSDDate(value) {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidDataType,
			Element: "infDPS/dCompet",
			Message: "dCompet must be a valid date in YYYY-MM-DD format",
			Value:   value,
		})
	}

	return errors
}

// validateTpEmit validates the tpEmit (emitter type) element.
func (v *XSDValidator) validateTpEmit(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	tpEmit := infDPS.FindElement("tpEmit")
	if tpEmit == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/tpEmit",
			Message: "required element 'tpEmit' (emitter type) not found",
		})
		return errors
	}

	value := strings.TrimSpace(tpEmit.Text())
	// Valid emitter types: 1 (provider), 2 (taker), 3 (intermediary)
	if value != "1" && value != "2" && value != "3" {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidValue,
			Element: "infDPS/tpEmit",
			Message: "tpEmit must be '1' (provider), '2' (taker), or '3' (intermediary)",
			Value:   value,
		})
	}

	return errors
}

// validateCLocEmi validates the cLocEmi (emission municipality code) element.
func (v *XSDValidator) validateCLocEmi(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	cLocEmi := infDPS.FindElement("cLocEmi")
	if cLocEmi == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/cLocEmi",
			Message: "required element 'cLocEmi' (emission municipality code) not found",
		})
		return errors
	}

	value := strings.TrimSpace(cLocEmi.Text())
	// IBGE municipality code: 7 digits
	if !regexp.MustCompile(`^\d{7}$`).MatchString(value) {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidFormat,
			Element: "infDPS/cLocEmi",
			Message: "cLocEmi must be exactly 7 digits (IBGE municipality code)",
			Value:   value,
		})
	}

	return errors
}

// validateSubst validates the subst (substitution) element.
func (v *XSDValidator) validateSubst(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	subst := infDPS.FindElement("subst")
	if subst == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/subst",
			Message: "required element 'subst' (substitution) not found",
		})
		return errors
	}

	value := strings.TrimSpace(subst.Text())
	// Valid substitution values: 1 (substitution), 2 (no substitution)
	if value != "1" && value != "2" {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidValue,
			Element: "infDPS/subst",
			Message: "subst must be '1' (substitution) or '2' (no substitution)",
			Value:   value,
		})
	}

	return errors
}

// validatePrest validates the prest (provider) element.
func (v *XSDValidator) validatePrest(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	prest := infDPS.FindElement("prest")
	if prest == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/prest",
			Message: "required element 'prest' (provider) not found",
		})
		return errors
	}

	// Validate CNPJ or CPF is present
	cnpj := prest.FindElement("CNPJ")
	cpf := prest.FindElement("CPF")
	if cnpj == nil && cpf == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/prest",
			Message: "provider must have either CNPJ or CPF",
		})
	}

	// Validate CNPJ format if present
	if cnpj != nil {
		value := strings.TrimSpace(cnpj.Text())
		if !regexp.MustCompile(`^\d{14}$`).MatchString(value) {
			errors = append(errors, XSDValidationError{
				Code:    XSDErrorInvalidFormat,
				Element: "infDPS/prest/CNPJ",
				Message: "CNPJ must be exactly 14 digits",
				Value:   value,
			})
		}
	}

	// Validate CPF format if present
	if cpf != nil {
		value := strings.TrimSpace(cpf.Text())
		if !regexp.MustCompile(`^\d{11}$`).MatchString(value) {
			errors = append(errors, XSDValidationError{
				Code:    XSDErrorInvalidFormat,
				Element: "infDPS/prest/CPF",
				Message: "CPF must be exactly 11 digits",
				Value:   value,
			})
		}
	}

	return errors
}

// validateServ validates the serv (service) element.
func (v *XSDValidator) validateServ(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	serv := infDPS.FindElement("serv")
	if serv == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serv",
			Message: "required element 'serv' (service) not found",
		})
		return errors
	}

	// Validate cTribNac (national service code)
	cTribNac := serv.FindElement("cTribNac")
	if cTribNac == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serv/cTribNac",
			Message: "required element 'cTribNac' (national service code) not found",
		})
	} else {
		value := strings.TrimSpace(cTribNac.Text())
		if !regexp.MustCompile(`^\d{6}$`).MatchString(value) {
			errors = append(errors, XSDValidationError{
				Code:    XSDErrorInvalidFormat,
				Element: "infDPS/serv/cTribNac",
				Message: "cTribNac must be exactly 6 digits",
				Value:   value,
			})
		}
	}

	// Validate xDescServ (service description)
	xDescServ := serv.FindElement("xDescServ")
	if xDescServ == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serv/xDescServ",
			Message: "required element 'xDescServ' (service description) not found",
		})
	} else {
		value := strings.TrimSpace(xDescServ.Text())
		if len(value) == 0 {
			errors = append(errors, XSDValidationError{
				Code:    XSDErrorInvalidValue,
				Element: "infDPS/serv/xDescServ",
				Message: "service description cannot be empty",
			})
		} else if len(value) > 2000 {
			errors = append(errors, XSDValidationError{
				Code:    XSDErrorInvalidValue,
				Element: "infDPS/serv/xDescServ",
				Message: "service description cannot exceed 2000 characters",
				Value:   fmt.Sprintf("%d characters", len(value)),
			})
		}
	}

	// Validate cLocPrest (service location municipality code)
	cLocPrest := serv.FindElement("cLocPrest")
	if cLocPrest == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serv/cLocPrest",
			Message: "required element 'cLocPrest' (service location municipality code) not found",
		})
	} else {
		value := strings.TrimSpace(cLocPrest.Text())
		if !regexp.MustCompile(`^\d{7}$`).MatchString(value) {
			errors = append(errors, XSDValidationError{
				Code:    XSDErrorInvalidFormat,
				Element: "infDPS/serv/cLocPrest",
				Message: "cLocPrest must be exactly 7 digits (IBGE municipality code)",
				Value:   value,
			})
		}
	}

	return errors
}

// validateValores validates the valores (values) element.
func (v *XSDValidator) validateValores(infDPS *etree.Element) []XSDValidationError {
	var errors []XSDValidationError

	valores := infDPS.FindElement("valores")
	if valores == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/valores",
			Message: "required element 'valores' (values) not found",
		})
		return errors
	}

	// Validate vServPrest (service value)
	vServPrest := valores.FindElement("vServPrest")
	if vServPrest == nil {
		// Try alternative element name
		vServPrest = valores.FindElement("vServ")
	}
	if vServPrest == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/valores/vServPrest",
			Message: "required element 'vServPrest' (service value) not found",
		})
	} else {
		value := strings.TrimSpace(vServPrest.Text())
		if !isValidDecimal(value) {
			errors = append(errors, XSDValidationError{
				Code:    XSDErrorInvalidDataType,
				Element: "infDPS/valores/vServPrest",
				Message: "vServPrest must be a valid decimal number",
				Value:   value,
			})
		} else {
			// Check if value is positive
			if val, err := strconv.ParseFloat(value, 64); err == nil && val <= 0 {
				errors = append(errors, XSDValidationError{
					Code:    XSDErrorInvalidValue,
					Element: "infDPS/valores/vServPrest",
					Message: "vServPrest must be greater than zero",
					Value:   value,
				})
			}
		}
	}

	return errors
}

// validateRequiredElement validates that a required element exists.
func (v *XSDValidator) validateRequiredElement(parent *etree.Element, elementName, description string) []XSDValidationError {
	var errors []XSDValidationError

	element := parent.FindElement(elementName)
	if element == nil {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorMissingElement,
			Element: fmt.Sprintf("%s/%s", parent.Tag, elementName),
			Message: fmt.Sprintf("required element '%s' (%s) not found", elementName, description),
		})
	} else if strings.TrimSpace(element.Text()) == "" {
		errors = append(errors, XSDValidationError{
			Code:    XSDErrorInvalidValue,
			Element: fmt.Sprintf("%s/%s", parent.Tag, elementName),
			Message: fmt.Sprintf("element '%s' (%s) cannot be empty", elementName, description),
		})
	}

	return errors
}

// isValidXSDDateTime checks if a string is a valid XSD dateTime (ISO 8601).
func isValidXSDDateTime(s string) bool {
	// Try various ISO 8601 formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05.000Z",
	}

	for _, format := range formats {
		if _, err := time.Parse(format, s); err == nil {
			return true
		}
	}

	return false
}

// isValidXSDDate checks if a string is a valid XSD date (YYYY-MM-DD).
func isValidXSDDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// isValidDecimal checks if a string is a valid decimal number.
func isValidDecimal(s string) bool {
	// Allow optional sign, digits, and optional decimal point with digits
	matched, _ := regexp.MatchString(`^-?\d+(\.\d+)?$`, s)
	return matched
}

// GetEnvironmentFromDPS extracts the environment type from a DPS XML.
// Returns 1 for production, 2 for homologation, or 0 if not found.
func GetEnvironmentFromDPS(dpsXML string) int {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(dpsXML); err != nil {
		return 0
	}

	tpAmb := doc.FindElement("//tpAmb")
	if tpAmb == nil {
		return 0
	}

	value := strings.TrimSpace(tpAmb.Text())
	if value == "1" {
		return 1
	}
	if value == "2" {
		return 2
	}

	return 0
}
