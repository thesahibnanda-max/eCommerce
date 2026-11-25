package usersService

type CreateUserRequest struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
}

type CreateUserResponse struct {
	ID           string `json:"id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type GetUserAndVerifyPasswordRequest struct {
	Email       string `json:"email"`
	PhoneNumber string `json:"phone_number"`
	Password    string `json:"password"`
}

type GetUserAndVerifyPasswordResponse struct {
	ID           string `json:"id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type VerifyAccessTokenRequest struct {
	AccessToken string `json:"access_token"`
}

type VerifyAccessTokenResponse struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Email                  string `json:"email"`
	PhoneNumber            string `json:"phone_number"`
	IsValidated            bool   `json:"is_validated"`
	IsEmailValidated       bool   `json:"is_email_validated"`
	IsPhoneNumberValidated bool   `json:"is_phone_number_validated"`
}

type RefreshAccessTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshAccessTokenResponse struct {
	ID           string `json:"id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type StartEmailValidationRequest struct {
	AccessToken string `json:"access_token"`
}

type StartEmailValidationResponse struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type ValidateEmailRequest struct {
	AccessToken string `json:"access_token"`
	OTP         string `json:"otp"`
}

type ValidateEmailResponse struct {
	ID          string `json:"id"`
	IsValidated bool   `json:"is_validated"`
}
