package utils_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestValidateAndNormalizeEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    utils.Email
		wantErr bool
	}{
		{
			name:  "simple address",
			input: "a@b.com",
			want:  utils.Email{UserName: "a", Domain: "b.com"},
		},
		{
			name:  "uppercase is lowercased",
			input: "John.Doe@Example.COM",
			want:  utils.Email{UserName: "john.doe", Domain: "example.com"},
		},
		{
			name:  "surrounding whitespace is trimmed",
			input: "  a@b.com  \n",
			want:  utils.Email{UserName: "a", Domain: "b.com"},
		},
		{
			name:  "plus tag and subdomains are kept",
			input: "user+tag@sub.example.co.in",
			want:  utils.Email{UserName: "user+tag", Domain: "sub.example.co.in"},
		},
		{
			name:  "display name is dropped",
			input: "John <J@X.com>",
			want:  utils.Email{UserName: "j", Domain: "x.com"},
		},
		{
			name:  "domain without tld is accepted",
			input: "a@localhost",
			want:  utils.Email{UserName: "a", Domain: "localhost"},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "missing at sign",
			input:   "plain",
			wantErr: true,
		},
		{
			name:    "missing domain",
			input:   "a@",
			wantErr: true,
		},
		{
			name:    "missing user name",
			input:   "@b.com",
			wantErr: true,
		},
		{
			name:    "space inside address",
			input:   "a b@c.com",
			wantErr: true,
		},
		{
			name:    "multiple addresses",
			input:   "a@b.com, c@d.com",
			wantErr: true,
		},
		{
			name:    "quoted local part containing at sign",
			input:   `"a@b"@c.com`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.ValidateAndNormalizeEmail(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateAndNormalizeEmail(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				if got != (utils.Email{}) {
					t.Errorf("ValidateAndNormalizeEmail(%q) = %+v on error, want zero value", tt.input, got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("ValidateAndNormalizeEmail(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidatePhoneNumber(t *testing.T) {
	indianMobile := utils.PhoneNumber{CountryCode: 91, Number: 9876543210}
	delhiLandline := utils.PhoneNumber{CountryCode: 91, Number: 1123456789}

	tests := []struct {
		name    string
		input   string
		want    utils.PhoneNumber
		wantErr bool
	}{
		{
			name:  "indian mobile without country code defaults to IN",
			input: "9876543210",
			want:  indianMobile,
		},
		{
			name:  "indian mobile with country code",
			input: "+919876543210",
			want:  indianMobile,
		},
		{
			name:  "indian mobile with spaces",
			input: "+91 98765 43210",
			want:  indianMobile,
		},
		{
			name:  "indian mobile with dash",
			input: "98765-43210",
			want:  indianMobile,
		},
		{
			name:  "indian mobile with trunk prefix zero",
			input: "09876543210",
			want:  indianMobile,
		},
		{
			name:  "surrounding whitespace is trimmed",
			input: "  9876543210  ",
			want:  indianMobile,
		},
		{
			name:  "indian landline with trunk prefix",
			input: "011 2345 6789",
			want:  delhiLandline,
		},
		{
			name:  "indian landline with country code",
			input: "+91 11 2345 6789",
			want:  delhiLandline,
		},
		{
			name:  "us number",
			input: "+1 650-253-0000",
			want:  utils.PhoneNumber{CountryCode: 1, Number: 6502530000},
		},
		{
			name:  "uk number",
			input: "+442070313000",
			want:  utils.PhoneNumber{CountryCode: 44, Number: 2070313000},
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "whitespace only",
			input:   "            ",
			wantErr: true,
		},
		{
			name:    "shorter than ten characters",
			input:   "12345",
			wantErr: true,
		},
		{
			name:    "short after trimming whitespace",
			input:   "     98765     ",
			wantErr: true,
		},
		{
			name:    "letters only",
			input:   "abcdefghij",
			wantErr: true,
		},
		{
			name:    "too many digits for india",
			input:   "+9198765432101",
			wantErr: true,
		},
		{
			name:    "ten characters but too few digits",
			input:   "+91 123 45",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := utils.ValidatePhoneNumber(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatePhoneNumber(%q) err = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				if got != (utils.PhoneNumber{}) {
					t.Errorf("ValidatePhoneNumber(%q) = %+v on error, want zero value", tt.input, got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("ValidatePhoneNumber(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{"letters digits symbol", "abcdef1!", nil},
		{"exactly 8 characters", "Ab1!Ab1!", nil},
		{"exactly 64 characters", strings.Repeat("a", 61) + "1!x", nil},
		{"spaces allowed inside", "correct horse 1!", nil},
		{"leading and trailing spaces count", " abc12!  ", nil},
		{"symbol instead of punctuation", "abcdef1+", nil},
		{"unicode letters", "pässwörd1!", nil},
		{"multibyte counted as characters", "密码密码密码1!", nil},
		{"emoji is a symbol", "abcdef1😀", nil},
		{"empty", "", utils.ErrPasswordTooShort},
		{"7 characters", "abcde1!", utils.ErrPasswordTooShort},
		{"7 multibyte characters", "密码密码密1!", utils.ErrPasswordTooShort},
		{"65 characters", strings.Repeat("a", 62) + "1!x", utils.ErrPasswordTooLong},
		{"no digit", "abcdefg!", utils.ErrPasswordTooWeak},
		{"no letter", "1234567!", utils.ErrPasswordTooWeak},
		{"no special", "abcdefg1", utils.ErrPasswordTooWeak},
		{"spaces are not special", "abcd efg1", utils.ErrPasswordTooWeak},
		{"superscript is not a digit", "abcdefg²!", utils.ErrPasswordTooWeak},
		{"only spaces", "          ", utils.ErrPasswordTooWeak},
		{"tab", "abc\tdef1!", utils.ErrPasswordInvalidCharacters},
		{"newline", "abcdef1!\n", utils.ErrPasswordInvalidCharacters},
		{"null byte", "abc\x00def1!", utils.ErrPasswordInvalidCharacters},
		{"invalid utf8", "abcdef1!\xff", utils.ErrPasswordInvalidCharacters},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := utils.ValidatePassword(tt.password)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidatePassword(%q) = %v, want nil", tt.password, err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, err, tt.wantErr)
			}
		})
	}
}
