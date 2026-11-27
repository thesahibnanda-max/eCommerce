package snapshotService

import "github.com/thesahibnanda-max/eCommerce/backend/package/utils"

const (
	ErrCtxInvalid      = utils.ErrContextInvalid
	ErrParamsInvalid   = Error("snapshotService: spreadsheet repository and at least one uniquely named source are required")
	ErrAlreadyRunning  = Error("snapshotService: a snapshot is already running")
	ErrDatabasesFailed = Error("snapshotService: some databases could not be snapshotted")
	ErrPanicked        = Error("snapshotService: recovered from panic")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
