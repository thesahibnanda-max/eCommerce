package usersRepository

import "time"

type Model struct {
	ID                     string         `json:"id"`
	Metadata               map[string]any `json:"metadata"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
	Name                   string         `json:"name"`
	Password               string         `json:"-"`
	Email                  string         `json:"email"`
	PhoneNumber            string         `json:"phone_number"`
	IsEmailValidated       bool           `json:"is_email_validated"`
	IsPhoneNumberValidated bool           `json:"is_phone_number_validated"`
}

type record struct {
	Model
	PasswordHash string `json:"password_hash"`
	PasswordSalt string `json:"password_salt"`
}
