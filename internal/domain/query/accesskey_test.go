package query

import (
	"errors"
	"testing"
)

func TestValidateAccessKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr error
	}{
		{
			name:    "valid access key",
			key:     "NFSe3550308202601081123456789012300000000000012310",
			wantErr: nil,
		},
		{
			name:    "valid access key with lowercase letters",
			key:     "NFSeabcd308202601081123456789012300000000abc012310",
			wantErr: nil,
		},
		{
			name:    "valid access key with mixed case",
			key:     "NFSe355030820260108112345678901230AbCdEfgh00012310",
			wantErr: nil,
		},
		{
			name:    "valid with whitespace trimmed",
			key:     "  NFSe3550308202601081123456789012300000000000012310  ",
			wantErr: nil,
		},
		{
			name:    "empty string",
			key:     "",
			wantErr: ErrAccessKeyEmpty,
		},
		{
			name:    "only whitespace",
			key:     "   ",
			wantErr: ErrAccessKeyEmpty,
		},
		{
			name:    "too short - 49 chars",
			key:     "NFSe355030820260108112345678901230000000000001231",
			wantErr: ErrAccessKeyInvalidLength,
		},
		{
			name:    "too long - 51 chars",
			key:     "NFSe35503082026010811234567890123000000000000123100",
			wantErr: ErrAccessKeyInvalidLength,
		},
		{
			name:    "wrong prefix - lowercase nfse",
			key:     "nfse3550308202601081123456789012300000000000012310",
			wantErr: ErrAccessKeyInvalidPrefix,
		},
		{
			name:    "wrong prefix - NFSE uppercase",
			key:     "NFSE3550308202601081123456789012300000000000012310",
			wantErr: ErrAccessKeyInvalidPrefix,
		},
		{
			name:    "wrong prefix - different prefix",
			key:     "DPS03550308202601081123456789012300000000000012310",
			wantErr: ErrAccessKeyInvalidPrefix,
		},
		{
			name:    "no prefix - starts with number",
			key:     "abcd3550308202601081123456789012300000000000012310",
			wantErr: ErrAccessKeyInvalidPrefix,
		},
		{
			name:    "contains special character hyphen",
			key:     "NFSe3550308202601081123456789-12300000000000012310",
			wantErr: ErrAccessKeyInvalidCharacters,
		},
		{
			name:    "contains special character underscore",
			key:     "NFSe3550308202601081123456789_12300000000000012310",
			wantErr: ErrAccessKeyInvalidCharacters,
		},
		{
			name:    "contains space in middle",
			key:     "NFSe3550308202601081123456789 12300000000000012310",
			wantErr: ErrAccessKeyInvalidCharacters,
		},
		{
			name:    "contains dot",
			key:     "NFSe3550308202601081123456789.12300000000000012310",
			wantErr: ErrAccessKeyInvalidCharacters,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAccessKey(tt.key)

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateAccessKey() unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("ValidateAccessKey() expected error %v, got nil", tt.wantErr)
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidateAccessKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidAccessKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{
			name: "valid key",
			key:  "NFSe3550308202601081123456789012300000000000012310",
			want: true,
		},
		{
			name: "invalid key - empty",
			key:  "",
			want: false,
		},
		{
			name: "invalid key - wrong prefix",
			key:  "XXXX3550308202601081123456789012300000000000012310",
			want: false,
		},
		{
			name: "invalid key - too short",
			key:  "NFSe123",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidAccessKey(tt.key); got != tt.want {
				t.Errorf("IsValidAccessKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeAccessKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		want    string
		wantErr bool
	}{
		{
			name:    "already normalized",
			key:     "NFSe3550308202601081123456789012300000000000012310",
			want:    "NFSe3550308202601081123456789012300000000000012310",
			wantErr: false,
		},
		{
			name:    "with leading/trailing whitespace",
			key:     "  NFSe3550308202601081123456789012300000000000012310  ",
			want:    "NFSe3550308202601081123456789012300000000000012310",
			wantErr: false,
		},
		{
			name:    "invalid key returns error",
			key:     "invalid",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeAccessKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("NormalizeAccessKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("NormalizeAccessKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseAccessKey(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		wantPrefix string
		wantBody   string
		wantErr    bool
	}{
		{
			name:       "valid key",
			key:        "NFSe3550308202601081123456789012300000000000012310",
			wantPrefix: "NFSe",
			wantBody:   "3550308202601081123456789012300000000000012310",
			wantErr:    false,
		},
		{
			name:    "invalid key",
			key:     "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseAccessKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAccessKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if got.Prefix != tt.wantPrefix {
				t.Errorf("ParseAccessKey().Prefix = %v, want %v", got.Prefix, tt.wantPrefix)
			}
			if got.Body != tt.wantBody {
				t.Errorf("ParseAccessKey().Body = %v, want %v", got.Body, tt.wantBody)
			}
		})
	}
}

func BenchmarkValidateAccessKey(b *testing.B) {
	key := "NFSe3550308202601081123456789012300000000000012310"
	for i := 0; i < b.N; i++ {
		ValidateAccessKey(key)
	}
}
