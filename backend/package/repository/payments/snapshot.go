package paymentsRepository

import (
	"context"

	dumpRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/dump"
)

func (r *repository) Dump(ctx context.Context) ([]dumpRepository.Table, error) {
	return dumpRepository.SQL(ctx, r.reader, []dumpRepository.TableSpec{{Name: "payments", Skip: []string{"proof_image"}}, {Name: "payment_events"}})
}
