package utils

import (
	"context"
	"fmt"
)

func CheckContext(ctx context.Context) error {
	if ctx == nil {
		return ErrContextInvalid
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%w: %w", ErrContextInvalid, err)
	}
	return nil
}
