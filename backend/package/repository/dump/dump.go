package dumpRepository

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

func Bolt(ctx context.Context, db *bolt.DB, opts BoltOptions) (tables []Table, err error) {
	defer recoverAsError(&err)
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	if db == nil {
		return nil, ErrDatabaseNil
	}
	err = db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, bucket *bolt.Bucket) error {
			if err := utils.CheckContext(ctx); err != nil {
				return err
			}
			if slices.Contains(opts.Skip, string(name)) {
				return nil
			}
			table := Table{Name: string(name), Rows: make([]Record, 0, bucket.Stats().KeyN)}
			if err := bucket.ForEach(func(key, value []byte) error {
				if value == nil {
					return nil
				}
				raw, err := encodeBoltValue(value, opts.Redact)
				if err != nil {
					return fmt.Errorf("%w: bucket %q: %w", ErrEncodingFailed, name, err)
				}
				table.Rows = append(table.Rows, Record{Key: encodeKey(key), JSON: raw})
				return nil
			}); err != nil {
				return err
			}
			tables = append(tables, table)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return tables, nil
}

func SQL(ctx context.Context, db *sql.DB, specs []TableSpec) (tables []Table, err error) {
	defer recoverAsError(&err)
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	if db == nil {
		return nil, ErrDatabaseNil
	}
	tables = make([]Table, 0, len(specs))
	for _, spec := range specs {
		if !identifierPattern.MatchString(spec.Name) {
			return nil, fmt.Errorf("%w: %q", ErrTableInvalid, spec.Name)
		}
		table, err := dumpTable(ctx, db, spec)
		if err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, nil
}
