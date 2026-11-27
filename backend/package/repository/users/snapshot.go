package usersRepository

import (
	"context"

	dumpRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/dump"
)

func (r *repository) Dump(ctx context.Context) ([]dumpRepository.Table, error) {
	return dumpRepository.Bolt(ctx, r.db, dumpRepository.BoltOptions{Redact: []string{"password_hash", "password_salt"}})
}
