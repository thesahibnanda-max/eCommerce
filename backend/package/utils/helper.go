package utils

import (
	"crypto/rand"
	"errors"
	"fmt"
)

func recoverAsError[T any](result *T, err *error) {
	if r := recover(); r != nil {
		var zero T
		*result = zero
		*err = fmt.Errorf("recovered from panic: %v", r)
	}
}

func recoverAsResult[T any](result *T) {
	if r := recover(); r != nil {
		var zero T
		*result = zero
	}
}

func randomBool() (bool, error) {
	var b [1]byte
	if _, err := rand.Read(b[:]); err != nil {
		return false, errors.Join(errors.New("failed to generate random sign"), err)
	}
	return b[0]&1 == 1, nil
}
