package cartsRepository

import (
	"time"
)

type Model struct {
	UserID    string    `json:"user_id"`
	Items     []Item    `json:"items"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Item struct {
	VariantID string    `json:"variant_id"`
	Quantity  int       `json:"quantity"`
	AddedAt   time.Time `json:"added_at"`
}
