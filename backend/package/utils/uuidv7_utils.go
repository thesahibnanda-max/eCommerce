package utils

import "github.com/google/uuid"

const uuidCanonicalLength = 36

func NewUUIDv7() (id string, err error) {
	defer recoverAsError(&id, &err)
	generated, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return generated.String(), nil
}

func ValidateUUIDv7(id string) bool {
	if len(id) != uuidCanonicalLength {
		return false
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		return false
	}
	return parsed.Version() == 7 && parsed.Variant() == uuid.RFC4122
}
