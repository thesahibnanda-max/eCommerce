package googleAuthService

import "time"

type Status struct {
	Connected   bool       `json:"connected"`
	Account     string     `json:"account,omitempty"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
}

type AuthRequest struct {
	CallbackURL string
	ReturnURL   string
}

type CallbackRequest struct {
	State string
	Code  string
	Error string
}
