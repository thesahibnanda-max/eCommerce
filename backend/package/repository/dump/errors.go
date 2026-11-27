package dumpRepository

import "github.com/thesahibnanda-max/eCommerce/backend/package/utils"

const (
	ErrCtxInvalid     = utils.ErrContextInvalid
	ErrDatabaseNil    = Error("dumpRepository: database handle is nil")
	ErrTableInvalid   = Error("dumpRepository: table name is invalid")
	ErrKeyMissing     = Error("dumpRepository: table has no id column")
	ErrReadFailed     = Error("dumpRepository: could not read database")
	ErrEncodingFailed = Error("dumpRepository: could not encode value as json")
	ErrPanicked       = Error("dumpRepository: recovered from panic")
)

type Error string

func (e Error) Error() string {
	return string(e)
}
