package utils

import (
	"fmt"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nyaruka/phonenumbers/v2"
)

const (
	minPasswordLength = 8
	maxPasswordLength = 64
)

type PhoneNumber struct {
	CountryCode uint
	Number      uint64
}

type Email struct {
	UserName string
	Domain   string
}

func ValidateAndNormalizeEmail(email string) (Email, error) {
	parsedEmail, err := mail.ParseAddress(strings.TrimSpace(strings.ToLower(email)))
	if err != nil {
		return Email{}, err
	}
	parts := strings.Split(parsedEmail.Address, "@")
	if len(parts) != 2 {
		return Email{}, fmt.Errorf("invalid email")
	}
	return Email{
		UserName: strings.TrimSpace(parts[0]),
		Domain:   strings.TrimSpace(parts[1]),
	}, nil
}

func ValidatePhoneNumber(phoneNumber string) (PhoneNumber, error) {
	phoneNumber = strings.ReplaceAll(phoneNumber, " ", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, "-", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, "(", "")
	phoneNumber = strings.ReplaceAll(phoneNumber, ")", "")
	if len(phoneNumber) < 10 {
		return PhoneNumber{}, fmt.Errorf("invalid phoneNumber")
	}

	num, err := phonenumbers.Parse(phoneNumber, "IN")
	if err != nil {
		return PhoneNumber{}, err
	}
	if num == nil {
		return PhoneNumber{}, fmt.Errorf("invalid phoneNumber")
	}
	if !phonenumbers.IsValidNumber(num) {
		return PhoneNumber{}, fmt.Errorf("invalid phoneNumber")
	}

	if num.CountryCode == nil || num.NationalNumber == nil {
		return PhoneNumber{}, fmt.Errorf("invalid phoneNumber")
	}

	return PhoneNumber{
		CountryCode: uint(*num.CountryCode),
		Number:      uint64(*num.NationalNumber),
	}, nil
}

func ValidatePassword(password string) error {
	if !utf8.ValidString(password) {
		return ErrPasswordInvalidCharacters
	}
	length := utf8.RuneCountInString(password)
	if length < minPasswordLength {
		return ErrPasswordTooShort
	}
	if length > maxPasswordLength {
		return ErrPasswordTooLong
	}

	hasLetter, hasDigit, hasSpecial := false, false, false
	for _, char := range password {
		switch {
		case unicode.IsControl(char):
			return ErrPasswordInvalidCharacters
		case unicode.IsLetter(char):
			hasLetter = true
		case unicode.IsDigit(char):
			hasDigit = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSpecial = true
		}
	}

	if !hasLetter || !hasDigit || !hasSpecial {
		return ErrPasswordTooWeak
	}
	return nil
}
