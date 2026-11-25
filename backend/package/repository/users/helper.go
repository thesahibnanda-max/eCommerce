package usersRepository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

func (r *repository) getUserByIndex(index []byte, key string) (Model, error) {
	var user Model
	if err := r.db.View(func(tx *bolt.Tx) error {
		id := tx.Bucket(index).Get([]byte(key))
		if id == nil {
			return ErrUserNotFound
		}
		var err error
		user, err = getUser(tx, string(id))
		if err == ErrUserNotFound {
			return ErrIndexCorrupted
		}
		return err
	}); err != nil {
		return Model{}, err
	}
	return user, nil
}

func (r *repository) getUserWithPassword(index []byte, key string, password string) (Model, error) {
	var rec record
	found := false
	if err := r.db.View(func(tx *bolt.Tx) error {
		id := tx.Bucket(index).Get([]byte(key))
		if id == nil {
			return nil
		}
		var err error
		rec, err = getRecord(tx, string(id))
		if err == ErrUserNotFound {
			return ErrIndexCorrupted
		}
		found = err == nil
		return err
	}); err != nil {
		return Model{}, err
	}
	if !found || rec.PasswordSalt == "" || rec.PasswordHash == "" {
		_ = verifyPassword(dummySalt, dummyHash, password)
		return Model{}, ErrInvalidCredentials
	}
	if !verifyPassword(rec.PasswordSalt, rec.PasswordHash, password) {
		return Model{}, ErrInvalidCredentials
	}
	return rec.Model, nil
}

func (r *repository) updateUser(ctx context.Context, id string, mutate func(user *Model) error) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(id) {
		return Model{}, ErrIDInvalid
	}

	var user Model
	if err := r.db.Update(func(tx *bolt.Tx) error {
		if err := utils.CheckContext(ctx); err != nil {
			return err
		}
		rec, err := getRecord(tx, id)
		if err != nil {
			return err
		}
		if err := mutate(&rec.Model); err != nil {
			return err
		}
		now, err := utils.TimeNow()
		if err != nil {
			return err
		}
		rec.UpdatedAt = now
		user = rec.Model
		return putRecord(tx, rec)
	}); err != nil {
		return Model{}, err
	}
	return user, nil
}

func getUser(tx *bolt.Tx, id string) (Model, error) {
	rec, err := getRecord(tx, id)
	if err != nil {
		return Model{}, err
	}
	return rec.Model, nil
}

func getRecord(tx *bolt.Tx, id string) (record, error) {
	raw := tx.Bucket(bucketUsers).Get([]byte(id))
	if raw == nil {
		return record{}, ErrUserNotFound
	}
	rec, err := utils.FromStringToObjectJSON[record](string(raw))
	if err != nil {
		return record{}, fmt.Errorf("usersRepository: decode user %s: %w", id, err)
	}
	if rec.Metadata == nil {
		rec.Metadata = make(map[string]any)
	}
	rec.Password = ""
	return rec, nil
}

func putRecord(tx *bolt.Tx, rec record) error {
	rec.Password = ""
	raw, err := utils.FromObjectToStringJSON(rec)
	if err != nil {
		return fmt.Errorf("usersRepository: encode user %s: %w", rec.ID, err)
	}
	return tx.Bucket(bucketUsers).Put([]byte(rec.ID), []byte(raw))
}

func normalizeEmail(email string) (string, error) {
	parsed, err := utils.ValidateAndNormalizeEmail(email)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrEmailInvalid, err)
	}
	return parsed.UserName + "@" + parsed.Domain, nil
}

func normalizePhoneNumber(phoneNumber string) (string, error) {
	parsed, err := utils.ValidatePhoneNumber(phoneNumber)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPhoneNumberInvalid, err)
	}
	return "+" + strconv.FormatUint(uint64(parsed.CountryCode), 10) + strconv.FormatUint(parsed.Number, 10), nil
}

func newPasswordSalt() (string, error) {
	salt := make([]byte, passwordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("usersRepository: generate password salt: %w", err)
	}
	return hex.EncodeToString(salt), nil
}

func hashPassword(salt string, password string) string {
	sum := sha256.Sum256([]byte(salt + password))
	return hex.EncodeToString(sum[:])
}

func verifyPassword(salt string, hash string, password string) bool {
	return subtle.ConstantTimeCompare([]byte(hashPassword(salt, password)), []byte(hash)) == 1
}
