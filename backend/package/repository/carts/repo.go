package cartsRepository

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketCarts = []byte("carts")
)

const openTimeout = time.Second

type Repository interface {
	GetCart(ctx context.Context, userID string) (Model, error)
	UpdateCart(ctx context.Context, userID string, mutate func(Model) (Model, error)) (Model, error)
	DeleteCart(ctx context.Context, userID string) error
	ListCarts(ctx context.Context, offset, limit int) ([]Model, int, error)
	Close() error
}

type repository struct {
	db  *bolt.DB
	now func() (time.Time, error)
}

func NewRepository(path []string, permission os.FileMode) (Repository, error) {
	db, err := bolt.Open(filepath.Join(path...), permission, &bolt.Options{Timeout: openTimeout})
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketCarts)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &repository{db: db, now: utils.TimeNow}, nil
}

func (r *repository) GetCart(ctx context.Context, userID string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(userID) {
		return Model{}, ErrUserIDInvalid
	}
	var cart Model
	if err := r.db.View(func(tx *bolt.Tx) error {
		var err error
		cart, err = getCart(tx, userID)
		return err
	}); err != nil {
		return Model{}, err
	}
	return cart, nil
}

func (r *repository) UpdateCart(ctx context.Context, userID string, mutate func(Model) (Model, error)) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(userID) {
		return Model{}, ErrUserIDInvalid
	}
	if mutate == nil {
		return Model{}, ErrMutateRequired
	}
	var cart Model
	if err := r.db.Update(func(tx *bolt.Tx) error {
		current, err := getCart(tx, userID)
		if err != nil {
			return err
		}
		next, err := runMutate(mutate, current)
		if err != nil {
			return err
		}
		next.UserID = userID
		if err := validateItems(next.Items); err != nil {
			return err
		}
		if err := utils.CheckContext(ctx); err != nil {
			return err
		}
		now, err := r.now()
		if err != nil {
			return err
		}
		next.UpdatedAt = now
		if err := putCart(tx, next); err != nil {
			return err
		}
		cart = next
		return nil
	}); err != nil {
		return Model{}, err
	}
	return cart, nil
}

func (r *repository) DeleteCart(ctx context.Context, userID string) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	if !utils.ValidateULID(userID) {
		return ErrUserIDInvalid
	}
	return r.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketCarts).Delete([]byte(userID))
	})
}

func (r *repository) Close() error {
	return r.db.Close()
}

const maxListLimit = 100

func (r *repository) ListCarts(ctx context.Context, offset, limit int) ([]Model, int, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, 0, err
	}
	if limit < 1 || limit > maxListLimit {
		return nil, 0, ErrLimitInvalid
	}
	if offset < 0 {
		return nil, 0, ErrOffsetInvalid
	}
	out := []Model{}
	total := 0
	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketCarts)
		total = bucket.Stats().KeyN
		cursor := bucket.Cursor()
		skipped := 0
		for k, _ := cursor.Last(); k != nil && len(out) < limit; k, _ = cursor.Prev() {
			if skipped < offset {
				skipped++
				continue
			}
			cart, err := getCart(tx, string(k))
			if err != nil {
				return err
			}
			out = append(out, cart)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
