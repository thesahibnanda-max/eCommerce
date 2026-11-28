package credentialsRepository

import "time"

type Model struct {
	Name      string
	Secret    string
	Account   string
	CreatedAt time.Time
}

type record struct {
	Account   string    `json:"account"`
	Sealed    []byte    `json:"sealed"`
	CreatedAt time.Time `json:"created_at"`
}
