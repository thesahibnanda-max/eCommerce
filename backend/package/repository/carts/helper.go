package cartsRepository

import (
	"fmt"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

func getCart(tx *bolt.Tx, userID string) (Model, error) {
	raw := tx.Bucket(bucketCarts).Get([]byte(userID))
	if raw == nil {
		return Model{UserID: userID, Items: []Item{}}, nil
	}
	cart, err := utils.FromStringToObjectJSON[Model](string(raw))
	if err != nil {
		return Model{}, err
	}
	if cart.Items == nil {
		cart.Items = []Item{}
	}
	return cart, nil
}

func putCart(tx *bolt.Tx, cart Model) error {
	bucket := tx.Bucket(bucketCarts)
	if len(cart.Items) == 0 {
		return bucket.Delete([]byte(cart.UserID))
	}
	raw, err := utils.FromObjectToStringJSON(cart)
	if err != nil {
		return err
	}
	return bucket.Put([]byte(cart.UserID), []byte(raw))
}

func validateItems(items []Item) error {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if !utils.ValidateULID(item.VariantID) {
			return ErrVariantIDInvalid
		}
		if item.Quantity < 1 {
			return ErrQuantityInvalid
		}
		if _, ok := seen[item.VariantID]; ok {
			return ErrItemDuplicate
		}
		seen[item.VariantID] = struct{}{}
	}
	return nil
}

func runMutate(mutate func(Model) (Model, error), current Model) (next Model, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			next = Model{}
			err = fmt.Errorf("%w: %v", ErrMutatePanicked, recovered)
		}
	}()
	snapshot := current
	snapshot.Items = append([]Item(nil), current.Items...)
	return mutate(snapshot)
}
