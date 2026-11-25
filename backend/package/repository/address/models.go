package addressRepository

import (
	"time"
)

type Model struct {
	ID           string         `json:"id"`
	UserID       string         `json:"userId"`
	Label        string         `json:"label"`
	FullName     string         `json:"fullName"`
	PhoneNumber  string         `json:"phoneNumber"`
	IsIndian     bool           `json:"isIndian"`
	IsDefault    bool           `json:"isDefault"`
	AddressLine1 string         `json:"addressLine1"`
	AddressLine2 string         `json:"addressLine2"`
	Landmark     string         `json:"landmark"`
	City         string         `json:"city"`
	State        string         `json:"state"`
	StateCode    string         `json:"stateCode"`
	Country      string         `json:"country"`
	Pincode      string         `json:"pincode"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
	Metadata     map[string]any `json:"metadata"`
}

type location struct {
	pincode   string
	city      string
	state     string
	stateCode string
}
