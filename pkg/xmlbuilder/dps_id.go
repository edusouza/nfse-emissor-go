// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

// Package xmlbuilder provides utilities for building NFS-e XML documents
// according to Brazilian government specifications.
package xmlbuilder

import (
	"fmt"
	"strings"
)

// DPSIDConfig contains the parameters needed to generate a DPS ID.
type DPSIDConfig struct {
	// MunicipalityCode is the 7-digit IBGE municipality code.
	MunicipalityCode string

	// RegistrationType indicates the type of federal registration:
	// 1 = CNPJ, 2 = CPF
	RegistrationType int

	// FederalRegistration is the CNPJ (14 digits) or CPF (11 digits) of the provider.
	FederalRegistration string

	// Series is the 5-digit DPS series.
	Series string

	// Number is the DPS number (will be zero-padded to 15 digits).
	Number string
}

// GenerateDPSID generates a DPS identification string according to the
// Sistema Nacional NFS-e specification.
//
// Format: DPS + MunCode(7) + RegType(1) + FedReg(14) + Series(5) + Number(15)
// Total length: 3 + 7 + 1 + 14 + 5 + 15 = 45 characters
//
// Example: DPS355030811234567800019500001000000000000001
func GenerateDPSID(config DPSIDConfig) (string, error) {
	// Validate municipality code (7 digits)
	if len(config.MunicipalityCode) != 7 {
		return "", fmt.Errorf("municipality code must be 7 digits, got %d", len(config.MunicipalityCode))
	}
	if !isAllDigits(config.MunicipalityCode) {
		return "", fmt.Errorf("municipality code must contain only digits")
	}

	// Validate registration type (1 = CNPJ, 2 = CPF)
	if config.RegistrationType != 1 && config.RegistrationType != 2 {
		return "", fmt.Errorf("registration type must be 1 (CNPJ) or 2 (CPF), got %d", config.RegistrationType)
	}

	// Validate and normalize federal registration
	fedReg := strings.ReplaceAll(config.FederalRegistration, ".", "")
	fedReg = strings.ReplaceAll(fedReg, "-", "")
	fedReg = strings.ReplaceAll(fedReg, "/", "")

	// Branch on the named constants, not on a literal: the two codes are the
	// reverse of what their names suggest, and a bare 1 here read as CNPJ while
	// the government reads it as CPF.
	switch config.RegistrationType {
	case RegistrationTypeCNPJ:
		if len(fedReg) != 14 {
			return "", fmt.Errorf("CNPJ must be 14 digits, got %d", len(fedReg))
		}
	case RegistrationTypeCPF:
		if len(fedReg) != 11 {
			return "", fmt.Errorf("CPF must be 11 digits, got %d", len(fedReg))
		}
		// The identifier carries 14 positions either way; a CPF is padded.
		fedReg = fmt.Sprintf("%014s", fedReg)
	default:
		return "", fmt.Errorf("registration type must be %d (CPF) or %d (CNPJ), got %d",
			RegistrationTypeCPF, RegistrationTypeCNPJ, config.RegistrationType)
	}

	if !isAllDigits(fedReg) {
		return "", fmt.Errorf("federal registration must contain only digits")
	}

	// Validate series (5 digits)
	if len(config.Series) != 5 {
		return "", fmt.Errorf("series must be 5 digits, got %d", len(config.Series))
	}
	if !isAllDigits(config.Series) {
		return "", fmt.Errorf("series must contain only digits")
	}

	// Validate and pad number to 15 digits
	number := strings.TrimLeft(config.Number, "0")
	if number == "" {
		number = "0"
	}
	if len(number) > 15 {
		return "", fmt.Errorf("number must be at most 15 digits, got %d", len(number))
	}
	if !isAllDigits(config.Number) {
		return "", fmt.Errorf("number must contain only digits")
	}
	paddedNumber := fmt.Sprintf("%015s", config.Number)

	// Build the DPS ID
	dpsID := fmt.Sprintf("DPS%s%d%s%s%s",
		config.MunicipalityCode,
		config.RegistrationType,
		fedReg,
		config.Series,
		paddedNumber,
	)

	return dpsID, nil
}

// ParseDPSID parses a DPS ID string and returns its components.
func ParseDPSID(dpsID string) (*DPSIDConfig, error) {
	// Check prefix
	if !strings.HasPrefix(dpsID, "DPS") {
		return nil, fmt.Errorf("DPS ID must start with 'DPS'")
	}

	// Check length (3 + 7 + 1 + 14 + 5 + 15 = 45)
	if len(dpsID) != 45 {
		return nil, fmt.Errorf("DPS ID must be 45 characters, got %d", len(dpsID))
	}

	// Extract components
	municipalityCode := dpsID[3:10]
	regType := int(dpsID[10] - '0')
	fedReg := dpsID[11:25]
	series := dpsID[25:30]
	number := dpsID[30:45]

	if regType != 1 && regType != 2 {
		return nil, fmt.Errorf("invalid registration type: %d", regType)
	}

	return &DPSIDConfig{
		MunicipalityCode:    municipalityCode,
		RegistrationType:    regType,
		FederalRegistration: fedReg,
		Series:              series,
		Number:              strings.TrimLeft(number, "0"),
	}, nil
}

// isAllDigits checks if a string contains only digit characters.
func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Registration type codes, as the government defines them in the E0004 rule of
// docs/anexos/ANEXO_I-SEFIN_ADN-DPS_NFSe-SNNFSe-v1.00-20251226.xlsx:
//
//	Tipo de inscrição Federal = 1 / Inscrição Federal = CPF emitente da DPS;
//	Tipo de inscrição Federal = 2 / Inscrição Federal = CNPJ emitente da DPS;
//
// CPF is 1 and CNPJ is 2 — the reverse of the order the names suggest, and of
// what this constant pair said until the government rejected every emission
// with E0004.
const (
	// RegistrationTypeCPF identifies an emitter by CPF (individual).
	RegistrationTypeCPF = 1

	// RegistrationTypeCNPJ identifies an emitter by CNPJ (company).
	RegistrationTypeCNPJ = 2
)
