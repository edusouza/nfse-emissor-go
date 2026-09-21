// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

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

// StructuralError represents a single XSD validation error.
type StructuralError struct {
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
func (e StructuralError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("%s [%s]: %s (value: %s)", e.Code, e.Element, e.Message, e.Value)
	}
	return fmt.Sprintf("%s [%s]: %s", e.Code, e.Element, e.Message)
}

// Expected namespace for NFS-e documents.
const (
	NFSeNamespace = "http://www.sped.fazenda.gov.br/nfse"
)

// StructuralValidator checks a DPS document for required elements, data types
// and formats.
//
// It is NOT an XSD validator: the .xsd files under docs/schemas are never read,
// so cardinality, element order, enumerations and patterns go unchecked. Treat
// it as a cheap first pass that catches obvious mistakes before a certificate
// and a network round-trip are spent; the authoritative validation is the
// government's. See https://github.com/edusouza/nfse-emissor-go/issues/4.
type StructuralValidator struct{}

// NewStructuralValidator creates a validator for DPS documents.
//
// It deliberately takes no arguments. The previous constructor accepted a
// schema directory that it stored and never used, which made the validation
// look broader than it is.
func NewStructuralValidator() *StructuralValidator {
	return &StructuralValidator{}
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
//   - []StructuralError: A slice of validation errors (empty if valid)
func (v *StructuralValidator) ValidateDPS(dpsXML string) []StructuralError {
	var errors []StructuralError

	// Parse the XML document
	doc := etree.NewDocument()
	if err := doc.ReadFromString(dpsXML); err != nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidFormat,
			Element: "document",
			Message: fmt.Sprintf("failed to parse XML: %v", err),
		})
		return errors
	}

	// Find the root DPS element
	dps := doc.Root()
	if dps == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "DPS",
			Message: "root DPS element not found",
		})
		return errors
	}

	// Validate root element name
	if dps.Tag != "DPS" {
		errors = append(errors, StructuralError{
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
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "DPS/infDPS",
			Message: "required element 'infDPS' not found",
		})
		return errors
	}

	// Validate infDPS has Id attribute
	if infDPS.SelectAttr("Id") == nil {
		errors = append(errors, StructuralError{
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
func (v *StructuralValidator) validateNamespace(dps *etree.Element) []StructuralError {
	var errors []StructuralError

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
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidNamespace,
			Element: "DPS",
			Message: fmt.Sprintf("DPS element should have namespace '%s'", NFSeNamespace),
		})
	}

	return errors
}

// validateInfDPS validates the infDPS element and its required children.
func (v *StructuralValidator) validateInfDPS(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	// Validate tpAmb (environment type) - required
	errors = append(errors, v.validateTpAmb(infDPS)...)

	// Validate dhEmi (emission date/time) - required
	errors = append(errors, v.validateDhEmi(infDPS)...)

	// verAplic has its own check: presence is not enough, the length matters.

	// Validate serie (series) - required
	errors = append(errors, v.validateVerAplic(infDPS)...)
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
func (v *StructuralValidator) validateTpAmb(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	tpAmb := infDPS.FindElement("tpAmb")
	if tpAmb == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/tpAmb",
			Message: "required element 'tpAmb' (environment type) not found",
		})
		return errors
	}

	value := strings.TrimSpace(tpAmb.Text())
	if value != "1" && value != "2" {
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidValue,
			Element: "infDPS/tpAmb",
			Message: "tpAmb must be '1' (production) or '2' (homologation)",
			Value:   value,
		})
	}

	return errors
}

// validateDhEmi validates the dhEmi (emission date/time) element.
func (v *StructuralValidator) validateDhEmi(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	dhEmi := infDPS.FindElement("dhEmi")
	if dhEmi == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/dhEmi",
			Message: "required element 'dhEmi' (emission date/time) not found",
		})
		return errors
	}

	value := strings.TrimSpace(dhEmi.Text())
	if !isValidXSDDateTime(value) {
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidDataType,
			Element: "infDPS/dhEmi",
			Message: "dhEmi must be a valid ISO 8601 datetime (e.g., 2024-01-15T10:30:00-03:00)",
			Value:   value,
		})
	}

	return errors
}

// validateSerie validates the serie element.
// maxVerAplicLength is the limit TSVerAplic sets on verAplic.
const maxVerAplicLength = 20

// validateVerAplic checks the application version field.
//
// It is easy to overflow without noticing: a Go pseudo-version alone is 40
// characters, and the government rejects the whole declaration over it.
func (v *StructuralValidator) validateVerAplic(infDPS *etree.Element) []StructuralError {
	el := infDPS.FindElement("verAplic")
	if el == nil {
		return []StructuralError{{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/verAplic",
			Message: "required element 'verAplic' (application version) not found",
		}}
	}

	value := strings.TrimSpace(el.Text())
	switch {
	case value == "":
		return []StructuralError{{
			Code:    XSDErrorInvalidValue,
			Element: "infDPS/verAplic",
			Message: "verAplic cannot be empty",
		}}
	case len(value) > maxVerAplicLength:
		return []StructuralError{{
			Code:    XSDErrorInvalidValue,
			Element: "infDPS/verAplic",
			Message: fmt.Sprintf("verAplic cannot exceed %d characters", maxVerAplicLength),
			Value:   fmt.Sprintf("%d characters", len(value)),
		}}
	}
	return nil
}

func (v *StructuralValidator) validateSerie(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	serie := infDPS.FindElement("serie")
	if serie == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serie",
			Message: "required element 'serie' not found",
		})
		return errors
	}

	value := strings.TrimSpace(serie.Text())
	// Series should be 5 digits
	if !regexp.MustCompile(`^\d{5}$`).MatchString(value) {
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidFormat,
			Element: "infDPS/serie",
			Message: "serie must be exactly 5 digits",
			Value:   value,
		})
	}

	return errors
}

// validateNDPS validates the nDPS (DPS number) element.
func (v *StructuralValidator) validateNDPS(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	nDPS := infDPS.FindElement("nDPS")
	if nDPS == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/nDPS",
			Message: "required element 'nDPS' (DPS number) not found",
		})
		return errors
	}

	value := strings.TrimSpace(nDPS.Text())
	// DPS number should be 1-15 digits
	if !regexp.MustCompile(`^\d{1,15}$`).MatchString(value) {
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidFormat,
			Element: "infDPS/nDPS",
			Message: "nDPS must be 1 to 15 digits",
			Value:   value,
		})
	}

	return errors
}

// validateDCompet validates the dCompet (competence date) element.
func (v *StructuralValidator) validateDCompet(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	dCompet := infDPS.FindElement("dCompet")
	if dCompet == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/dCompet",
			Message: "required element 'dCompet' (competence date) not found",
		})
		return errors
	}

	value := strings.TrimSpace(dCompet.Text())
	if !isValidXSDDate(value) {
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidDataType,
			Element: "infDPS/dCompet",
			Message: "dCompet must be a valid date in YYYY-MM-DD format",
			Value:   value,
		})
	}

	return errors
}

// validateTpEmit validates the tpEmit (emitter type) element.
func (v *StructuralValidator) validateTpEmit(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	tpEmit := infDPS.FindElement("tpEmit")
	if tpEmit == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/tpEmit",
			Message: "required element 'tpEmit' (emitter type) not found",
		})
		return errors
	}

	value := strings.TrimSpace(tpEmit.Text())
	// Valid emitter types: 1 (provider), 2 (taker), 3 (intermediary)
	if value != "1" && value != "2" && value != "3" {
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidValue,
			Element: "infDPS/tpEmit",
			Message: "tpEmit must be '1' (provider), '2' (taker), or '3' (intermediary)",
			Value:   value,
		})
	}

	return errors
}

// validateCLocEmi validates the cLocEmi (emission municipality code) element.
func (v *StructuralValidator) validateCLocEmi(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	cLocEmi := infDPS.FindElement("cLocEmi")
	if cLocEmi == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/cLocEmi",
			Message: "required element 'cLocEmi' (emission municipality code) not found",
		})
		return errors
	}

	value := strings.TrimSpace(cLocEmi.Text())
	// IBGE municipality code: 7 digits
	if !regexp.MustCompile(`^\d{7}$`).MatchString(value) {
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidFormat,
			Element: "infDPS/cLocEmi",
			Message: "cLocEmi must be exactly 7 digits (IBGE municipality code)",
			Value:   value,
		})
	}

	return errors
}

// validateSubst validates the subst (substitution) element.
func (v *StructuralValidator) validateSubst(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	// subst is optional (minOccurs="0") and is present only when this DPS
	// replaces an existing NFS-e. It is a structure, not a yes/no flag: the
	// previous code required it and expected the text "1" or "2".
	subst := infDPS.FindElement("subst")
	if subst == nil {
		return nil
	}

	for _, req := range []struct{ name, desc string }{
		{"chSubstda", "access key of the NFS-e being replaced"},
		{"cMotivo", "substitution reason code"},
	} {
		el := subst.FindElement(req.name)
		if el == nil || strings.TrimSpace(el.Text()) == "" {
			errors = append(errors, StructuralError{
				Code:    XSDErrorMissingElement,
				Element: "infDPS/subst/" + req.name,
				Message: fmt.Sprintf("required element for %s not found", req.desc),
			})
		}
	}

	return errors
}

// validatePrest validates the prest (provider) element.
func (v *StructuralValidator) validatePrest(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	prest := infDPS.FindElement("prest")
	if prest == nil {
		errors = append(errors, StructuralError{
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
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/prest",
			Message: "provider must have either CNPJ or CPF",
		})
	}

	// Validate CNPJ format if present
	if cnpj != nil {
		value := strings.TrimSpace(cnpj.Text())
		if !regexp.MustCompile(`^\d{14}$`).MatchString(value) {
			errors = append(errors, StructuralError{
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
			errors = append(errors, StructuralError{
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
func (v *StructuralValidator) validateServ(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	serv := infDPS.FindElement("serv")
	if serv == nil {
		return append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serv",
			Message: "required element 'serv' (service) not found",
		})
	}

	// Paths follow TCServ / TCCServ / TCLocPrest in tiposComplexos_v1.00.xsd:
	// cTribNac and xDescServ are children of cServ, and the municipality code
	// is cLocPrestacao inside locPrest.
	if cTribNac := serv.FindElement("cServ/cTribNac"); cTribNac == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serv/cServ/cTribNac",
			Message: "required element 'cTribNac' (national service code) not found",
		})
	} else {
		value := strings.TrimSpace(cTribNac.Text())
		if !regexp.MustCompile(`^\d{6}$`).MatchString(value) {
			errors = append(errors, StructuralError{
				Code:    XSDErrorInvalidFormat,
				Element: "infDPS/serv/cServ/cTribNac",
				Message: "cTribNac must be exactly 6 digits",
				Value:   value,
			})
		}
	}

	if xDescServ := serv.FindElement("cServ/xDescServ"); xDescServ == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serv/cServ/xDescServ",
			Message: "required element 'xDescServ' (service description) not found",
		})
	} else {
		value := strings.TrimSpace(xDescServ.Text())
		switch {
		case len(value) == 0:
			errors = append(errors, StructuralError{
				Code:    XSDErrorInvalidValue,
				Element: "infDPS/serv/cServ/xDescServ",
				Message: "service description cannot be empty",
			})
		case len(value) > 2000:
			errors = append(errors, StructuralError{
				Code:    XSDErrorInvalidValue,
				Element: "infDPS/serv/cServ/xDescServ",
				Message: "service description cannot exceed 2000 characters",
				Value:   fmt.Sprintf("%d characters", len(value)),
			})
		}
	}

	// locPrest is a choice: cLocPrestacao for a domestic service, or
	// cPaisPrestacao for one provided abroad. Exactly one must be present.
	cLocPrestacao := serv.FindElement("locPrest/cLocPrestacao")
	cPaisPrestacao := serv.FindElement("locPrest/cPaisPrestacao")

	switch {
	case cLocPrestacao == nil && cPaisPrestacao == nil:
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/serv/locPrest",
			Message: "locPrest must contain either 'cLocPrestacao' or 'cPaisPrestacao'",
		})
	case cLocPrestacao != nil && cPaisPrestacao != nil:
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidValue,
			Element: "infDPS/serv/locPrest",
			Message: "locPrest accepts only one of 'cLocPrestacao' or 'cPaisPrestacao'",
		})
	case cLocPrestacao != nil:
		value := strings.TrimSpace(cLocPrestacao.Text())
		if !regexp.MustCompile(`^\d{7}$`).MatchString(value) {
			errors = append(errors, StructuralError{
				Code:    XSDErrorInvalidFormat,
				Element: "infDPS/serv/locPrest/cLocPrestacao",
				Message: "cLocPrestacao must be exactly 7 digits (IBGE municipality code)",
				Value:   value,
			})
		}
	}

	return errors
}

// validateValores validates the valores (values) element.
func (v *StructuralValidator) validateValores(infDPS *etree.Element) []StructuralError {
	var errors []StructuralError

	valores := infDPS.FindElement("valores")
	if valores == nil {
		return append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/valores",
			Message: "required element 'valores' (values) not found",
		})
	}

	// vServPrest is a container (TCVServPrest); the amount is in its vServ child.
	if vServ := valores.FindElement("vServPrest/vServ"); vServ == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/valores/vServPrest/vServ",
			Message: "required element 'vServ' (service value) not found",
		})
	} else {
		value := strings.TrimSpace(vServ.Text())
		if !isValidDecimal(value) {
			errors = append(errors, StructuralError{
				Code:    XSDErrorInvalidDataType,
				Element: "infDPS/valores/vServPrest/vServ",
				Message: "vServ must be a valid decimal number",
				Value:   value,
			})
		} else if val, err := strconv.ParseFloat(value, 64); err == nil && val <= 0 {
			errors = append(errors, StructuralError{
				Code:    XSDErrorInvalidValue,
				Element: "infDPS/valores/vServPrest/vServ",
				Message: "vServ must be greater than zero",
				Value:   value,
			})
		}
	}

	// tribMun carries two mandatory codes that are easy to omit.
	for _, req := range []struct{ path, desc string }{
		{"trib/tribMun/tribISSQN", "ISSQN taxation type"},
		{"trib/tribMun/tpRetISSQN", "ISSQN withholding type"},
	} {
		if valores.FindElement(req.path) == nil {
			errors = append(errors, StructuralError{
				Code:    XSDErrorMissingElement,
				Element: "infDPS/valores/" + req.path,
				Message: fmt.Sprintf("required element for %s not found", req.desc),
			})
		}
	}

	// totTrib is a choice of exactly one child.
	if totTrib := valores.FindElement("trib/totTrib"); totTrib == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: "infDPS/valores/trib/totTrib",
			Message: "required element 'totTrib' (total taxes) not found",
		})
	} else if n := len(totTrib.ChildElements()); n != 1 {
		errors = append(errors, StructuralError{
			Code:    XSDErrorInvalidValue,
			Element: "infDPS/valores/trib/totTrib",
			Message: "totTrib must contain exactly one of vTotTrib, pTotTrib, indTotTrib or pTotTribSN",
			Value:   fmt.Sprintf("%d elements", n),
		})
	}

	return errors
}

// validateRequiredElement validates that a required element exists.
func (v *StructuralValidator) validateRequiredElement(parent *etree.Element, elementName, description string) []StructuralError {
	var errors []StructuralError

	element := parent.FindElement(elementName)
	if element == nil {
		errors = append(errors, StructuralError{
			Code:    XSDErrorMissingElement,
			Element: fmt.Sprintf("%s/%s", parent.Tag, elementName),
			Message: fmt.Sprintf("required element '%s' (%s) not found", elementName, description),
		})
	} else if strings.TrimSpace(element.Text()) == "" {
		errors = append(errors, StructuralError{
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
