package utils

import "github.com/oklog/ulid/v2"

func NewULID() string {
	return ulid.Make().String()
}

func ValidateULID(id string) bool {
	_, err := ulid.ParseStrict(id)
	return err == nil
}
