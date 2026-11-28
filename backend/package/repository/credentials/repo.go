package credentialsRepository

import (
	"context"
	"crypto/cipher"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

var bucketCredentials = []byte("credentials")

const openTimeout = time.Second

type Repository interface {
	Get(ctx context.Context, name string) (Model, error)
	Put(ctx context.Context, model Model) error
	Delete(ctx context.Context, name string) error
	Close() error
}

type repository struct {
	db   *bolt.DB
	aead cipher.AEAD
}

func NewRepository(path []string, permission os.FileMode, encryptionKey string) (Repository, error) {
	if strings.TrimSpace(encryptionKey) == "" {
		return nil, ErrKeyRequired
	}
	aead, err := newAEAD(encryptionKey)
	if err != nil {
		return nil, err
	}
	db, err := bolt.Open(filepath.Join(path...), permission, &bolt.Options{Timeout: openTimeout})
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketCredentials)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &repository{db: db, aead: aead}, nil
}

func (r *repository) Get(ctx context.Context, name string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Model{}, ErrNameInvalid
	}
	var raw string
	if err := r.db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket(bucketCredentials).Get([]byte(name))
		if value == nil {
			return ErrNotFound
		}
		raw = string(value)
		return nil
	}); err != nil {
		return Model{}, err
	}
	stored, err := utils.FromStringToObjectJSON[record](raw)
	if err != nil {
		return Model{}, ErrRecordCorrupted
	}
	secret, err := r.open(name, stored.Sealed)
	if err != nil {
		return Model{}, err
	}
	return Model{Name: name, Secret: secret, Account: stored.Account, CreatedAt: stored.CreatedAt}, nil
}

func (r *repository) Put(ctx context.Context, model Model) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	name := strings.TrimSpace(model.Name)
	if name == "" {
		return ErrNameInvalid
	}
	if model.Secret == "" {
		return ErrSecretRequired
	}
	sealed, err := r.seal(name, model.Secret)
	if err != nil {
		return err
	}
	raw, err := utils.FromObjectToStringJSON(record{Account: model.Account, Sealed: sealed, CreatedAt: model.CreatedAt})
	if err != nil {
		return err
	}
	return r.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketCredentials).Put([]byte(name), []byte(raw))
	})
}

func (r *repository) Delete(ctx context.Context, name string) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ErrNameInvalid
	}
	return r.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketCredentials).Delete([]byte(name))
	})
}

func (r *repository) Close() error {
	return r.db.Close()
}
