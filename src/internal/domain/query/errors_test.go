package query

import (
	"errors"
	"net/http"
	"testing"
)

func TestQueryErrorCode_HTTPStatus(t *testing.T) {
	tests := []struct {
		code   QueryErrorCode
		status int
	}{
		{ErrorCodeInvalidAccessKey, http.StatusBadRequest},
		{ErrorCodeInvalidDPSID, http.StatusBadRequest},
		{ErrorCodeCertificateRequired, http.StatusBadRequest},
		{ErrorCodeCertificateInvalid, http.StatusBadRequest},
		{ErrorCodeForbiddenAccess, http.StatusForbidden},
		{ErrorCodeNFSeNotFound, http.StatusNotFound},
		{ErrorCodeDPSNotFound, http.StatusNotFound},
		{ErrorCodeGovernmentUnavailable, http.StatusServiceUnavailable},
		{ErrorCodeGovernmentTimeout, http.StatusGatewayTimeout},
		{QueryErrorCode("UNKNOWN"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.HTTPStatus(); got != tt.status {
				t.Errorf("HTTPStatus() = %v, want %v", got, tt.status)
			}
		})
	}
}

func TestQueryErrorCode_IsRetryable(t *testing.T) {
	tests := []struct {
		code      QueryErrorCode
		retryable bool
	}{
		{ErrorCodeInvalidAccessKey, false},
		{ErrorCodeInvalidDPSID, false},
		{ErrorCodeNFSeNotFound, false},
		{ErrorCodeDPSNotFound, false},
		{ErrorCodeForbiddenAccess, false},
		{ErrorCodeCertificateRequired, false},
		{ErrorCodeCertificateInvalid, false},
		{ErrorCodeGovernmentUnavailable, true},
		{ErrorCodeGovernmentTimeout, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if got := tt.code.IsRetryable(); got != tt.retryable {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.retryable)
			}
		})
	}
}

func TestQueryError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *QueryError
		want string
	}{
		{
			name: "without detail",
			err: &QueryError{
				Code:    ErrorCodeNFSeNotFound,
				Message: "NFS-e not found",
			},
			want: "NFSE_NOT_FOUND: NFS-e not found",
		},
		{
			name: "with detail",
			err: &QueryError{
				Code:    ErrorCodeInvalidAccessKey,
				Message: "Invalid access key",
				Detail:  "Key must be 50 characters",
			},
			want: "INVALID_ACCESS_KEY: Invalid access key (Key must be 50 characters)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQueryError_Is(t *testing.T) {
	err1 := &QueryError{Code: ErrorCodeNFSeNotFound}
	err2 := &QueryError{Code: ErrorCodeNFSeNotFound}
	err3 := &QueryError{Code: ErrorCodeDPSNotFound}

	if !errors.Is(err1, err2) {
		t.Error("errors.Is should return true for same code")
	}

	if errors.Is(err1, err3) {
		t.Error("errors.Is should return false for different codes")
	}

	if errors.Is(err1, errors.New("other error")) {
		t.Error("errors.Is should return false for non-QueryError")
	}
}

func TestNewQueryError(t *testing.T) {
	err := NewQueryError(ErrorCodeGovernmentTimeout, "Timeout occurred")

	if err.Code != ErrorCodeGovernmentTimeout {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeGovernmentTimeout)
	}
	if err.Message != "Timeout occurred" {
		t.Errorf("Message = %v, want %v", err.Message, "Timeout occurred")
	}
	if !err.Retryable {
		t.Error("Retryable should be true for timeout errors")
	}
}

func TestNewQueryErrorWithDetail(t *testing.T) {
	err := NewQueryErrorWithDetail(ErrorCodeInvalidAccessKey, "Invalid key", "Check format")

	if err.Code != ErrorCodeInvalidAccessKey {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeInvalidAccessKey)
	}
	if err.Message != "Invalid key" {
		t.Errorf("Message = %v, want %v", err.Message, "Invalid key")
	}
	if err.Detail != "Check format" {
		t.Errorf("Detail = %v, want %v", err.Detail, "Check format")
	}
}

func TestNewQueryErrorFromGovernment(t *testing.T) {
	err := NewQueryErrorFromGovernment(ErrorCodeNFSeNotFound, "Q020", "NFS-e nao encontrada")

	if err.Code != ErrorCodeNFSeNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeNFSeNotFound)
	}
	if err.GovernmentCode != "Q020" {
		t.Errorf("GovernmentCode = %v, want %v", err.GovernmentCode, "Q020")
	}
	if err.Message != "NFS-e nao encontrada" {
		t.Errorf("Message = %v, want %v", err.Message, "NFS-e nao encontrada")
	}
}

func TestIsQueryError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "QueryError",
			err:  &QueryError{Code: ErrorCodeNFSeNotFound},
			want: true,
		},
		{
			name: "wrapped QueryError",
			err:  errors.Join(errors.New("wrapper"), &QueryError{Code: ErrorCodeNFSeNotFound}),
			want: true,
		},
		{
			name: "regular error",
			err:  errors.New("regular error"),
			want: false,
		},
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsQueryError(tt.err); got != tt.want {
				t.Errorf("IsQueryError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetQueryError(t *testing.T) {
	qe := &QueryError{Code: ErrorCodeNFSeNotFound}

	got, ok := GetQueryError(qe)
	if !ok {
		t.Error("GetQueryError should return true for QueryError")
	}
	if got.Code != ErrorCodeNFSeNotFound {
		t.Errorf("Code = %v, want %v", got.Code, ErrorCodeNFSeNotFound)
	}

	_, ok = GetQueryError(errors.New("not a query error"))
	if ok {
		t.Error("GetQueryError should return false for non-QueryError")
	}
}

func TestTranslateQueryCode(t *testing.T) {
	tests := []struct {
		code       string
		wantNil    bool
		wantMapped QueryErrorCode
	}{
		{"Q001", false, ErrorCodeInvalidAccessKey},
		{"Q020", false, ErrorCodeNFSeNotFound},
		{"Q040", false, ErrorCodeForbiddenAccess},
		{"Q060", false, ErrorCodeCertificateRequired},
		{"Q100", false, ErrorCodeGovernmentUnavailable},
		{"q001", false, ErrorCodeInvalidAccessKey}, // lowercase
		{" Q001 ", false, ErrorCodeInvalidAccessKey}, // with spaces
		{"UNKNOWN", true, ""},
		{"", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := TranslateQueryCode(tt.code)
			if tt.wantNil {
				if got != nil {
					t.Errorf("TranslateQueryCode() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Error("TranslateQueryCode() = nil, want non-nil")
				return
			}
			if got.MappedCode != tt.wantMapped {
				t.Errorf("MappedCode = %v, want %v", got.MappedCode, tt.wantMapped)
			}
		})
	}
}

func TestTranslateQueryCodeWithDefault(t *testing.T) {
	// Known code
	got := TranslateQueryCodeWithDefault("Q020", "NFS-e nao encontrada")
	if got.MappedCode != ErrorCodeNFSeNotFound {
		t.Errorf("MappedCode = %v, want %v", got.MappedCode, ErrorCodeNFSeNotFound)
	}

	// Unknown code
	got = TranslateQueryCodeWithDefault("UNKNOWN", "Some error message")
	if got.Code != "UNKNOWN" {
		t.Errorf("Code = %v, want %v", got.Code, "UNKNOWN")
	}
	if got.Message != "Some error message" {
		t.Errorf("Message = %v, want %v", got.Message, "Some error message")
	}
	if got.Category != CategoryQueryUnknown {
		t.Errorf("Category = %v, want %v", got.Category, CategoryQueryUnknown)
	}
}

func TestTranslateToQueryError(t *testing.T) {
	err := TranslateToQueryError("Q020", "NFS-e nao encontrada")

	if err.Code != ErrorCodeNFSeNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeNFSeNotFound)
	}
	if err.GovernmentCode != "Q020" {
		t.Errorf("GovernmentCode = %v, want %v", err.GovernmentCode, "Q020")
	}
	if err.Retryable {
		t.Error("Retryable should be false for not found errors")
	}
}

func TestIsRetryableQueryCode(t *testing.T) {
	tests := []struct {
		code      string
		retryable bool
	}{
		{"Q001", false},
		{"Q020", false},
		{"Q040", false},
		{"Q100", true},
		{"Q101", true},
		{"Q102", true},
		{"UNKNOWN", false},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := IsRetryableQueryCode(tt.code); got != tt.retryable {
				t.Errorf("IsRetryableQueryCode(%q) = %v, want %v", tt.code, got, tt.retryable)
			}
		})
	}
}

func TestGetQueryCategory(t *testing.T) {
	tests := []struct {
		code     string
		category QueryErrorCategory
	}{
		{"Q001", CategoryQueryValidation},
		{"Q020", CategoryQueryNotFound},
		{"Q040", CategoryQueryPermission},
		{"Q060", CategoryQueryCertificate},
		{"Q100", CategoryQueryService},
		{"UNKNOWN", CategoryQueryUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := GetQueryCategory(tt.code); got != tt.category {
				t.Errorf("GetQueryCategory(%q) = %v, want %v", tt.code, got, tt.category)
			}
		})
	}
}

func TestRegisterQueryCode(t *testing.T) {
	// Register a custom code
	customCode := GovernmentQueryCode{
		Code:        "QCUSTOM",
		Message:     "Custom error",
		Description: "A custom error for testing",
		Action:      "Handle it",
		Category:    CategoryQueryValidation,
		Retryable:   false,
		MappedCode:  ErrorCodeInvalidAccessKey,
	}

	RegisterQueryCode(customCode)

	// Verify it was registered
	got := TranslateQueryCode("QCUSTOM")
	if got == nil {
		t.Fatal("TranslateQueryCode() returned nil for registered code")
	}
	if got.Description != "A custom error for testing" {
		t.Errorf("Description = %v, want %v", got.Description, "A custom error for testing")
	}
}

func TestGetAllQueryCodes(t *testing.T) {
	codes := GetAllQueryCodes()
	if len(codes) == 0 {
		t.Error("GetAllQueryCodes() returned empty map")
	}

	// Verify known codes exist
	if _, ok := codes["Q001"]; !ok {
		t.Error("Q001 not found in codes")
	}
	if _, ok := codes["Q020"]; !ok {
		t.Error("Q020 not found in codes")
	}
}

func TestGovernmentQueryCode_FormatForAPI(t *testing.T) {
	code := &GovernmentQueryCode{
		Code:        "Q020",
		Message:     "NFS-e nao encontrada",
		Description: "The NFS-e was not found",
		Action:      "Verify the access key",
		Retryable:   false,
		MappedCode:  ErrorCodeNFSeNotFound,
	}

	formatted := code.FormatForAPI()

	if formatted.Code != "NFSE_NOT_FOUND" {
		t.Errorf("Code = %v, want %v", formatted.Code, "NFSE_NOT_FOUND")
	}
	if formatted.Title != "NFS-e nao encontrada" {
		t.Errorf("Title = %v, want %v", formatted.Title, "NFS-e nao encontrada")
	}
	if formatted.GovernmentCode != "Q020" {
		t.Errorf("GovernmentCode = %v, want %v", formatted.GovernmentCode, "Q020")
	}
}

func TestFormatQueryErrorForAPI(t *testing.T) {
	err := &QueryError{
		Code:           ErrorCodeNFSeNotFound,
		Message:        "NFS-e not found",
		Detail:         "Verify access key",
		GovernmentCode: "Q020",
		Retryable:      false,
	}

	formatted := FormatQueryErrorForAPI(err)

	if formatted.Code != "NFSE_NOT_FOUND" {
		t.Errorf("Code = %v, want %v", formatted.Code, "NFSE_NOT_FOUND")
	}
	if formatted.Title != "NFS-e not found" {
		t.Errorf("Title = %v, want %v", formatted.Title, "NFS-e not found")
	}
	if formatted.Description != "Verify access key" {
		t.Errorf("Description = %v, want %v", formatted.Description, "Verify access key")
	}
}

func TestPredefinedErrors(t *testing.T) {
	// Test that predefined errors have correct codes
	tests := []struct {
		err  *QueryError
		code QueryErrorCode
	}{
		{ErrInvalidAccessKeyFormat, ErrorCodeInvalidAccessKey},
		{ErrInvalidDPSIDFormat, ErrorCodeInvalidDPSID},
		{ErrNFSeNotFound, ErrorCodeNFSeNotFound},
		{ErrDPSNotFound, ErrorCodeDPSNotFound},
		{ErrForbiddenAccess, ErrorCodeForbiddenAccess},
		{ErrCertificateRequired, ErrorCodeCertificateRequired},
		{ErrCertificateInvalid, ErrorCodeCertificateInvalid},
		{ErrGovernmentUnavailable, ErrorCodeGovernmentUnavailable},
		{ErrGovernmentTimeout, ErrorCodeGovernmentTimeout},
	}

	for _, tt := range tests {
		t.Run(string(tt.code), func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Code = %v, want %v", tt.err.Code, tt.code)
			}
		})
	}
}

func TestNewInvalidAccessKeyError(t *testing.T) {
	err := NewInvalidAccessKeyError("must be 50 characters")

	if err.Code != ErrorCodeInvalidAccessKey {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeInvalidAccessKey)
	}
	if err.Detail != "must be 50 characters" {
		t.Errorf("Detail = %v, want %v", err.Detail, "must be 50 characters")
	}
	if err.Retryable {
		t.Error("Retryable should be false")
	}
	if err.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus() = %v, want %v", err.HTTPStatus(), http.StatusBadRequest)
	}
}

func TestNewInvalidDPSIDError(t *testing.T) {
	err := NewInvalidDPSIDError("must be 42 numeric characters")

	if err.Code != ErrorCodeInvalidDPSID {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeInvalidDPSID)
	}
	if err.Detail != "must be 42 numeric characters" {
		t.Errorf("Detail = %v, want %v", err.Detail, "must be 42 numeric characters")
	}
	if err.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus() = %v, want %v", err.HTTPStatus(), http.StatusBadRequest)
	}
}

func TestNewNFSeNotFoundError(t *testing.T) {
	err := NewNFSeNotFoundError()

	if err.Code != ErrorCodeNFSeNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeNFSeNotFound)
	}
	if err.HTTPStatus() != http.StatusNotFound {
		t.Errorf("HTTPStatus() = %v, want %v", err.HTTPStatus(), http.StatusNotFound)
	}
	if err.Retryable {
		t.Error("Retryable should be false")
	}
}

func TestNewDPSNotFoundError(t *testing.T) {
	err := NewDPSNotFoundError()

	if err.Code != ErrorCodeDPSNotFound {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeDPSNotFound)
	}
	if err.HTTPStatus() != http.StatusNotFound {
		t.Errorf("HTTPStatus() = %v, want %v", err.HTTPStatus(), http.StatusNotFound)
	}
}

func TestNewForbiddenAccessError(t *testing.T) {
	err := NewForbiddenAccessError()

	if err.Code != ErrorCodeForbiddenAccess {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeForbiddenAccess)
	}
	if err.HTTPStatus() != http.StatusForbidden {
		t.Errorf("HTTPStatus() = %v, want %v", err.HTTPStatus(), http.StatusForbidden)
	}
}

func TestNewCertificateRequiredError(t *testing.T) {
	err := NewCertificateRequiredError()

	if err.Code != ErrorCodeCertificateRequired {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeCertificateRequired)
	}
	if err.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus() = %v, want %v", err.HTTPStatus(), http.StatusBadRequest)
	}
}

func TestNewCertificateInvalidError(t *testing.T) {
	err := NewCertificateInvalidError("certificate expired on 2024-01-01")

	if err.Code != ErrorCodeCertificateInvalid {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeCertificateInvalid)
	}
	if err.Detail != "certificate expired on 2024-01-01" {
		t.Errorf("Detail = %v, want %v", err.Detail, "certificate expired on 2024-01-01")
	}
	if err.HTTPStatus() != http.StatusBadRequest {
		t.Errorf("HTTPStatus() = %v, want %v", err.HTTPStatus(), http.StatusBadRequest)
	}
}

func TestNewGovernmentUnavailableError(t *testing.T) {
	err := NewGovernmentUnavailableError()

	if err.Code != ErrorCodeGovernmentUnavailable {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeGovernmentUnavailable)
	}
	if err.HTTPStatus() != http.StatusServiceUnavailable {
		t.Errorf("HTTPStatus() = %v, want %v", err.HTTPStatus(), http.StatusServiceUnavailable)
	}
	if !err.Retryable {
		t.Error("Retryable should be true for government unavailable errors")
	}
}

func TestNewGovernmentTimeoutError(t *testing.T) {
	err := NewGovernmentTimeoutError()

	if err.Code != ErrorCodeGovernmentTimeout {
		t.Errorf("Code = %v, want %v", err.Code, ErrorCodeGovernmentTimeout)
	}
	if err.HTTPStatus() != http.StatusGatewayTimeout {
		t.Errorf("HTTPStatus() = %v, want %v", err.HTTPStatus(), http.StatusGatewayTimeout)
	}
	if !err.Retryable {
		t.Error("Retryable should be true for government timeout errors")
	}
}

func TestTranslateGovernmentError(t *testing.T) {
	tests := []struct {
		name       string
		govCode    string
		govMessage string
		wantCode   QueryErrorCode
		retryable  bool
	}{
		{
			name:       "Q-prefix code Q020",
			govCode:    "Q020",
			govMessage: "NFS-e nao encontrada",
			wantCode:   ErrorCodeNFSeNotFound,
			retryable:  false,
		},
		{
			name:       "Q-prefix code Q100 retryable",
			govCode:    "Q100",
			govMessage: "Servico indisponivel",
			wantCode:   ErrorCodeGovernmentUnavailable,
			retryable:  true,
		},
		{
			name:       "E-prefix code E001",
			govCode:    "E001",
			govMessage: "Dados invalidos",
			wantCode:   ErrorCodeInvalidAccessKey,
			retryable:  false,
		},
		{
			name:       "E-prefix code E002",
			govCode:    "E002",
			govMessage: "Nao encontrado",
			wantCode:   ErrorCodeNFSeNotFound,
			retryable:  false,
		},
		{
			name:       "E-prefix code E003",
			govCode:    "E003",
			govMessage: "Acesso negado",
			wantCode:   ErrorCodeForbiddenAccess,
			retryable:  false,
		},
		{
			name:       "E-prefix code E004 retryable",
			govCode:    "E004",
			govMessage: "Servico indisponivel",
			wantCode:   ErrorCodeGovernmentUnavailable,
			retryable:  true,
		},
		{
			name:       "lowercase e-prefix",
			govCode:    "e001",
			govMessage: "Dados invalidos",
			wantCode:   ErrorCodeInvalidAccessKey,
			retryable:  false,
		},
		{
			name:       "unknown code",
			govCode:    "X999",
			govMessage: "Unknown error occurred",
			wantCode:   ErrorCodeGovernmentUnavailable,
			retryable:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := TranslateGovernmentError(tt.govCode, tt.govMessage)

			if err.Code != tt.wantCode {
				t.Errorf("Code = %v, want %v", err.Code, tt.wantCode)
			}
			if err.Retryable != tt.retryable {
				t.Errorf("Retryable = %v, want %v", err.Retryable, tt.retryable)
			}
			if err.GovernmentCode != tt.govCode {
				t.Errorf("GovernmentCode = %v, want %v", err.GovernmentCode, tt.govCode)
			}
		})
	}
}

func TestTranslateGovernmentError_PreservesOriginalMessage(t *testing.T) {
	// For E-prefix codes, the original message should be in Detail
	err := TranslateGovernmentError("E001", "Original Portuguese message")
	if err.Detail != "Original Portuguese message" {
		t.Errorf("Detail = %v, want %v", err.Detail, "Original Portuguese message")
	}

	// For unknown codes, the original message should also be preserved
	err = TranslateGovernmentError("UNKNOWN", "Some error message")
	if err.Detail != "Some error message" {
		t.Errorf("Detail = %v, want %v", err.Detail, "Some error message")
	}
}
