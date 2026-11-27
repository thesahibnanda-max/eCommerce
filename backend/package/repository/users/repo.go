package usersRepository

import (
	"context"
	"fmt"
	dumpRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/dump"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketUsers              = []byte("users")
	bucketUsersByEmail       = []byte("users_by_email")
	bucketUsersByPhoneNumber = []byte("users_by_phone_number")
)

const openTimeout = time.Second

type Repository interface {
	CreateUser(ctx context.Context, user Model) (Model, error)
	GetUserByID(ctx context.Context, id string) (Model, error)
	GetUserByEmail(ctx context.Context, email string) (Model, error)
	GetUserByPhoneNumber(ctx context.Context, phoneNumber string) (Model, error)
	ValidateEmail(ctx context.Context, id string) (Model, error)
	ValidatePhoneNumber(ctx context.Context, id string) (Model, error)
	UpdateMetadata(ctx context.Context, id string, metadata map[string]any) (Model, error)
	GetUserByEmailAndPassword(ctx context.Context, email string, password string) (Model, error)
	GetUserByPhoneNumberAndPassword(ctx context.Context, phoneNumber string, password string) (Model, error)
	ListUsers(ctx context.Context, offset, limit int) ([]Model, int, error)
	Dump(ctx context.Context) ([]dumpRepository.Table, error)
	Close() error
}

type repository struct {
	db *bolt.DB
}

func NewRepository(path []string, permission os.FileMode) (Repository, error) {
	db, err := bolt.Open(filepath.Join(path...), permission, &bolt.Options{Timeout: openTimeout})
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketUsers, bucketUsersByEmail, bucketUsersByPhoneNumber} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &repository{
		db: db,
	}, nil
}

func (r *repository) CreateUser(ctx context.Context, user Model) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}

	user.Name = strings.TrimSpace(user.Name)
	if user.Name == "" {
		return Model{}, ErrNameRequired
	}

	if strings.TrimSpace(user.Email) == "" {
		return Model{}, ErrEmailRequired
	}
	if strings.TrimSpace(user.PhoneNumber) == "" {
		return Model{}, ErrPhoneNumberRequired
	}

	if err := utils.ValidatePassword(user.Password); err != nil {
		return Model{}, fmt.Errorf("%w: %w", ErrPasswordInvalid, err)
	}
	salt, err := newPasswordSalt()
	if err != nil {
		return Model{}, err
	}
	rec := record{PasswordSalt: salt, PasswordHash: hashPassword(salt, user.Password)}
	user.Password = ""

	email, err := normalizeEmail(user.Email)
	if err != nil {
		return Model{}, err
	}
	user.Email = email

	phoneNumber, err := normalizePhoneNumber(user.PhoneNumber)
	if err != nil {
		return Model{}, err
	}
	user.PhoneNumber = phoneNumber

	now, err := utils.TimeNow()
	if err != nil {
		return Model{}, err
	}
	user.ID = utils.NewULID()
	if user.Metadata == nil {
		user.Metadata = make(map[string]any)
	}
	user.CreatedAt = now
	user.UpdatedAt = now
	user.IsEmailValidated = false
	user.IsPhoneNumberValidated = false
	rec.Model = user

	if err := r.db.Update(func(tx *bolt.Tx) error {
		if err := utils.CheckContext(ctx); err != nil {
			return err
		}

		byEmail := tx.Bucket(bucketUsersByEmail)
		byPhoneNumber := tx.Bucket(bucketUsersByPhoneNumber)

		if byEmail.Get([]byte(user.Email)) != nil {
			return ErrEmailAlreadyExists
		}
		if byPhoneNumber.Get([]byte(user.PhoneNumber)) != nil {
			return ErrPhoneNumberAlreadyExists
		}
		if err := byEmail.Put([]byte(user.Email), []byte(user.ID)); err != nil {
			return err
		}
		if err := byPhoneNumber.Put([]byte(user.PhoneNumber), []byte(user.ID)); err != nil {
			return err
		}

		return putRecord(tx, rec)
	}); err != nil {
		return Model{}, err
	}

	return user, nil
}

func (r *repository) GetUserByID(ctx context.Context, id string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(id) {
		return Model{}, ErrIDInvalid
	}

	var user Model
	if err := r.db.View(func(tx *bolt.Tx) error {
		var err error
		user, err = getUser(tx, id)
		return err
	}); err != nil {
		return Model{}, err
	}
	return user, nil
}

func (r *repository) GetUserByEmail(ctx context.Context, email string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	normalized, err := normalizeEmail(email)
	if err != nil {
		return Model{}, err
	}
	return r.getUserByIndex(bucketUsersByEmail, normalized)
}

func (r *repository) GetUserByPhoneNumber(ctx context.Context, phoneNumber string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	normalized, err := normalizePhoneNumber(phoneNumber)
	if err != nil {
		return Model{}, err
	}
	return r.getUserByIndex(bucketUsersByPhoneNumber, normalized)
}

func (r *repository) ValidateEmail(ctx context.Context, id string) (Model, error) {
	return r.updateUser(ctx, id, func(user *Model) error {
		user.IsEmailValidated = true
		return nil
	})
}

func (r *repository) ValidatePhoneNumber(ctx context.Context, id string) (Model, error) {
	return r.updateUser(ctx, id, func(user *Model) error {
		user.IsPhoneNumberValidated = true
		return nil
	})
}

func (r *repository) UpdateMetadata(ctx context.Context, id string, metadata map[string]any) (Model, error) {
	return r.updateUser(ctx, id, func(user *Model) error {
		user.Metadata = utils.UnionOfMaps(user.Metadata, metadata)
		return nil
	})
}

func (r *repository) GetUserByEmailAndPassword(ctx context.Context, email string, password string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	normalized, err := normalizeEmail(email)
	if err != nil {
		return Model{}, ErrInvalidCredentials
	}
	return r.getUserWithPassword(bucketUsersByEmail, normalized, password)
}

func (r *repository) GetUserByPhoneNumberAndPassword(ctx context.Context, phoneNumber string, password string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	normalized, err := normalizePhoneNumber(phoneNumber)
	if err != nil {
		return Model{}, ErrInvalidCredentials
	}
	return r.getUserWithPassword(bucketUsersByPhoneNumber, normalized, password)
}

func (r *repository) Close() error {
	return r.db.Close()
}

const maxListLimit = 100

func (r *repository) ListUsers(ctx context.Context, offset, limit int) ([]Model, int, error) {
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
		bucket := tx.Bucket(bucketUsers)
		total = bucket.Stats().KeyN
		cursor := bucket.Cursor()
		skipped := 0
		for k, _ := cursor.Last(); k != nil && len(out) < limit; k, _ = cursor.Prev() {
			if skipped < offset {
				skipped++
				continue
			}
			user, err := getUser(tx, string(k))
			if err != nil {
				return err
			}
			out = append(out, user)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
