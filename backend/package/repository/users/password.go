package usersRepository

import (
	"encoding/hex"
)

const passwordSaltBytes = 16

var (
	dummySalt = hex.EncodeToString(make([]byte, passwordSaltBytes))
	dummyHash = hashPassword(dummySalt, "")
)
