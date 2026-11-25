package addressesService

import (
	"time"
)

type AddressInput struct {
	Label        string `json:"label"`
	FullName     string `json:"full_name"`
	PhoneNumber  string `json:"phone_number"`
	AddressLine1 string `json:"address_line1"`
	AddressLine2 string `json:"address_line2"`
	Landmark     string `json:"landmark"`
	City         string `json:"city"`
	Pincode      string `json:"pincode"`
	IsDefault    bool   `json:"is_default"`
}

type Address struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	FullName     string    `json:"full_name"`
	PhoneNumber  string    `json:"phone_number"`
	AddressLine1 string    `json:"address_line1"`
	AddressLine2 string    `json:"address_line2"`
	Landmark     string    `json:"landmark"`
	City         string    `json:"city"`
	State        string    `json:"state"`
	StateCode    string    `json:"state_code"`
	Country      string    `json:"country"`
	Pincode      string    `json:"pincode"`
	IsDefault    bool      `json:"is_default"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ListAddressesRequest struct {
	AccessToken string `json:"access_token"`
}

type ListAddressesResponse struct {
	Addresses []Address `json:"addresses"`
}

type CreateAddressRequest struct {
	AccessToken string       `json:"access_token"`
	Address     AddressInput `json:"address"`
}

type UpdateAddressRequest struct {
	AccessToken string       `json:"access_token"`
	ID          string       `json:"id"`
	Address     AddressInput `json:"address"`
}

type DeleteAddressRequest struct {
	AccessToken string `json:"access_token"`
	ID          string `json:"id"`
}

type DeleteAddressResponse struct {
	ID string `json:"id"`
}

type SetDefaultAddressRequest struct {
	AccessToken string `json:"access_token"`
	ID          string `json:"id"`
}

type AddressResponse struct {
	Address Address `json:"address"`
}
