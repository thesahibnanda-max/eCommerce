package handler

import (
	cartsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/carts"
)

type IDsBody struct {
	IDs []string `json:"ids"`
}

type ValidateEmailBody struct {
	OTP string `json:"otp"`
}

type QuantityBody struct {
	Quantity int `json:"quantity"`
}

type MergeCartBody struct {
	Items []cartsService.ItemInput `json:"items"`
}

type PlaceOrderBody struct {
	AddressID      string `json:"address_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type TransitionOrderBody struct {
	To     string `json:"to"`
	Reason string `json:"reason"`
}

type DecidePaymentBody struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

type TrackingBody struct {
	Courier string `json:"courier"`
	Number  string `json:"number"`
	URL     string `json:"url"`
}
