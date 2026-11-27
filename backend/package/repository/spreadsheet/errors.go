package spreadsheetRepository

import "github.com/thesahibnanda-max/eCommerce/backend/package/utils"

const (
	ErrCtxInvalid         = utils.ErrContextInvalid
	ErrConfigInvalid      = Error("spreadsheetRepository: client id, client secret, refresh token and a positive timeout are required")
	ErrClosed             = Error("spreadsheetRepository: repository is closed")
	ErrDatabaseInvalid    = Error("spreadsheetRepository: database name is required and must be at most 80 characters")
	ErrSheetsRequired     = Error("spreadsheetRepository: snapshot needs at least one sheet")
	ErrSheetNameInvalid   = Error("spreadsheetRepository: sheet name must be 1-100 characters without [ ] : * ? / \\")
	ErrSheetNameDuplicate = Error("spreadsheetRepository: sheet names must be unique")
	ErrRowInvalid         = Error("spreadsheetRepository: row needs a key and valid json")
	ErrCellTooLarge       = Error("spreadsheetRepository: cell is over the 50000 character limit of google sheets")
	ErrTakenAtRequired    = Error("spreadsheetRepository: snapshot time is required")
	ErrRequestFailed      = Error("spreadsheetRepository: google api request failed")
	ErrPanicked           = Error("spreadsheetRepository: recovered from panic")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
