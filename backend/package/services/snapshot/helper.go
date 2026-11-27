package snapshotService

import "fmt"

const emptySheetName = "empty"

func recoverAsError(err *error) {
	if recovered := recover(); recovered != nil {
		*err = fmt.Errorf("%w: %v", ErrPanicked, recovered)
	}
}
