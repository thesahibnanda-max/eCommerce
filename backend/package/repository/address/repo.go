package addressRepository

import (
	"context"
	dumpRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/dump"
	"os"
	"path/filepath"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketAddresses = []byte("addresses")
	bucketUserIndex = []byte("addresses_by_user")
	bucketDefaults  = []byte("address_defaults")
)

const openTimeout = time.Second

type Repository interface {
	CreateAddress(ctx context.Context, address Model) (Model, error)
	GetAddress(ctx context.Context, userID, addressID string) (Model, error)
	ListAddresses(ctx context.Context, userID string) ([]Model, error)
	CountAddresses(ctx context.Context, userID string) (int, error)
	GetDefaultAddress(ctx context.Context, userID string) (Model, error)
	UpdateAddress(ctx context.Context, userID, addressID string, mutate func(Model) (Model, error)) (Model, error)
	SetDefaultAddress(ctx context.Context, userID, addressID string) (Model, error)
	DeleteAddress(ctx context.Context, userID, addressID string) error
	DeleteAllAddresses(ctx context.Context, userID string) (int, error)
	Dump(ctx context.Context) ([]dumpRepository.Table, error)
	Close() error
}

type repository struct {
	db  *bolt.DB
	now func() (time.Time, error)
}

func NewRepository(path []string, permission os.FileMode) (Repository, error) {
	if _, err := loadPincodes(); err != nil {
		return nil, err
	}
	db, err := bolt.Open(filepath.Join(path...), permission, &bolt.Options{Timeout: openTimeout})
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketAddresses, bucketUserIndex, bucketDefaults} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &repository{db: db, now: utils.TimeNow}, nil
}

func (r *repository) CreateAddress(ctx context.Context, address Model) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(address.UserID) {
		return Model{}, ErrUserIDInvalid
	}
	normalized, err := normalize(address, nil)
	if err != nil {
		return Model{}, err
	}
	now, err := r.now()
	if err != nil {
		return Model{}, err
	}
	normalized.ID = utils.NewULID()
	normalized.CreatedAt = now
	normalized.UpdatedAt = now
	if err := r.db.Update(func(tx *bolt.Tx) error {
		count := len(addressIDsOf(tx, normalized.UserID))
		if count >= maxAddressesPerUser {
			return ErrAddressLimitReached
		}
		if err := utils.CheckContext(ctx); err != nil {
			return err
		}
		if err := putAddress(tx, normalized); err != nil {
			return err
		}
		if count == 0 || address.IsDefault {
			normalized.IsDefault = true
			return setDefault(tx, normalized.UserID, normalized.ID)
		}
		normalized.IsDefault = false
		return nil
	}); err != nil {
		return Model{}, err
	}
	return normalized, nil
}

func (r *repository) GetAddress(ctx context.Context, userID, addressID string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validateIDs(userID, addressID); err != nil {
		return Model{}, err
	}
	var address Model
	if err := r.db.View(func(tx *bolt.Tx) error {
		var err error
		address, err = getAddress(tx, userID, addressID)
		return err
	}); err != nil {
		return Model{}, err
	}
	return address, nil
}

func (r *repository) ListAddresses(ctx context.Context, userID string) ([]Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	if !utils.ValidateULID(userID) {
		return nil, ErrUserIDInvalid
	}
	var addresses []Model
	if err := r.db.View(func(tx *bolt.Tx) error {
		var err error
		addresses, err = listAddresses(tx, userID)
		return err
	}); err != nil {
		return nil, err
	}
	return addresses, nil
}

func (r *repository) CountAddresses(ctx context.Context, userID string) (int, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return 0, err
	}
	if !utils.ValidateULID(userID) {
		return 0, ErrUserIDInvalid
	}
	count := 0
	if err := r.db.View(func(tx *bolt.Tx) error {
		count = len(addressIDsOf(tx, userID))
		return nil
	}); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *repository) GetDefaultAddress(ctx context.Context, userID string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(userID) {
		return Model{}, ErrUserIDInvalid
	}
	var address Model
	if err := r.db.View(func(tx *bolt.Tx) error {
		id := tx.Bucket(bucketDefaults).Get([]byte(userID))
		if id == nil {
			return ErrNoDefaultAddress
		}
		var err error
		address, err = getAddress(tx, userID, string(id))
		if err == ErrAddressNotFound {
			return ErrIndexCorrupted
		}
		return err
	}); err != nil {
		return Model{}, err
	}
	return address, nil
}

func (r *repository) UpdateAddress(ctx context.Context, userID, addressID string, mutate func(Model) (Model, error)) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validateIDs(userID, addressID); err != nil {
		return Model{}, err
	}
	if mutate == nil {
		return Model{}, ErrMutateRequired
	}
	var address Model
	if err := r.db.Update(func(tx *bolt.Tx) error {
		current, err := getAddress(tx, userID, addressID)
		if err != nil {
			return err
		}
		next, err := runMutate(mutate, current)
		if err != nil {
			return err
		}
		next, err = normalize(next, &current)
		if err != nil {
			return err
		}
		if err := utils.CheckContext(ctx); err != nil {
			return err
		}
		now, err := r.now()
		if err != nil {
			return err
		}
		next.ID = current.ID
		next.UserID = current.UserID
		next.CreatedAt = current.CreatedAt
		next.IsDefault = current.IsDefault
		next.UpdatedAt = now
		if err := putAddress(tx, next); err != nil {
			return err
		}
		address = next
		return nil
	}); err != nil {
		return Model{}, err
	}
	return address, nil
}

func (r *repository) SetDefaultAddress(ctx context.Context, userID, addressID string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if err := validateIDs(userID, addressID); err != nil {
		return Model{}, err
	}
	var address Model
	if err := r.db.Update(func(tx *bolt.Tx) error {
		var err error
		address, err = getAddress(tx, userID, addressID)
		if err != nil {
			return err
		}
		if address.IsDefault {
			return nil
		}
		if err := setDefault(tx, userID, addressID); err != nil {
			return err
		}
		address.IsDefault = true
		return nil
	}); err != nil {
		return Model{}, err
	}
	return address, nil
}

func (r *repository) DeleteAddress(ctx context.Context, userID, addressID string) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	if err := validateIDs(userID, addressID); err != nil {
		return err
	}
	return r.db.Update(func(tx *bolt.Tx) error {
		address, err := getAddress(tx, userID, addressID)
		if err != nil {
			return err
		}
		if err := deleteAddress(tx, userID, addressID); err != nil {
			return err
		}
		if address.IsDefault {
			return promoteNewest(tx, userID)
		}
		return nil
	})
}

func (r *repository) DeleteAllAddresses(ctx context.Context, userID string) (int, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return 0, err
	}
	if !utils.ValidateULID(userID) {
		return 0, ErrUserIDInvalid
	}
	deleted := 0
	if err := r.db.Update(func(tx *bolt.Tx) error {
		ids := addressIDsOf(tx, userID)
		for _, id := range ids {
			if err := deleteAddress(tx, userID, id); err != nil {
				return err
			}
		}
		if err := tx.Bucket(bucketDefaults).Delete([]byte(userID)); err != nil {
			return err
		}
		deleted = len(ids)
		return nil
	}); err != nil {
		return 0, err
	}
	return deleted, nil
}

func (r *repository) Close() error {
	return r.db.Close()
}
