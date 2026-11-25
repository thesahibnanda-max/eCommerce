package utils

import (
	"crypto/rand"
	"fmt"
	"strings"
)

const (
	maxRandomNumberLength = 128
	digitRejectThreshold  = 250
)

func GenerateRandomNumberWithLength(length uint, onlyPositive bool) (string, error) {
	if length == 0 || length > maxRandomNumberLength {
		return "", ErrRandomNumberLength
	}

	digits := make([]byte, 0, length)
	buf := make([]byte, length+8)
	for uint(len(digits)) < length {
		if _, err := rand.Read(buf); err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		for _, b := range buf {
			if b >= digitRejectThreshold {
				continue
			}
			digits = append(digits, '0'+b%10)
			if uint(len(digits)) == length {
				break
			}
		}
	}

	number := string(digits)
	if onlyPositive || strings.Trim(number, "0") == "" {
		return number, nil
	}

	negative, err := randomBool()
	if err != nil {
		return "", err
	}
	if negative {
		return "-" + number, nil
	}
	return number, nil
}
