package addressRepository

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

const (
	countryIndia        = "India"
	maxAddressesPerUser = 20
	maxFullNameRunes    = 100
	maxLineRunes        = 200
	maxLandmarkRunes    = 120
	maxCityRunes        = 80
	maxLabelRunes       = 30
	maxMetadataBytes    = 4096
)

//go:embed pincodes.csv
var pincodesCSV string

var (
	pincodePattern = regexp.MustCompile(`^[1-9][0-9]{5}$`)
	pincodesOnce   sync.Once
	pincodes       map[string]location
	pincodesErr    error
)

var countryAliases = map[string]struct{}{
	"":                  {},
	"india":             {},
	"in":                {},
	"ind":               {},
	"bharat":            {},
	"republic of india": {},
}

func userIndexKey(userID, addressID string) []byte {
	return []byte(userID + addressID)
}

func loadPincodes() (map[string]location, error) {
	pincodesOnce.Do(func() {
		pincodes, pincodesErr = parsePincodes(pincodesCSV)
	})
	return pincodes, pincodesErr
}

func parsePincodes(raw string) (table map[string]location, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			table = nil
			err = fmt.Errorf("%w: %v", ErrPincodeLookupFailed, recovered)
		}
	}()
	reader := csv.NewReader(strings.NewReader(raw))
	reader.FieldsPerRecord = 4
	reader.ReuseRecord = true
	if _, err := reader.Read(); err != nil {
		return nil, fmt.Errorf("%w: header: %w", ErrPincodeLookupFailed, err)
	}
	table = make(map[string]location, 20000)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrPincodeLookupFailed, err)
		}
		loc := location{pincode: row[0], city: row[1], state: row[2], stateCode: row[3]}
		if !pincodePattern.MatchString(loc.pincode) || loc.city == "" || loc.state == "" || loc.stateCode == "" {
			return nil, fmt.Errorf("%w: bad row for %q", ErrPincodeLookupFailed, loc.pincode)
		}
		if _, exists := table[loc.pincode]; exists {
			return nil, fmt.Errorf("%w: duplicate pincode %q", ErrPincodeLookupFailed, loc.pincode)
		}
		table[loc.pincode] = loc
	}
	if len(table) == 0 {
		return nil, fmt.Errorf("%w: no pincodes", ErrPincodeLookupFailed)
	}
	return table, nil
}

func lookupPincode(pin string) (location, error) {
	pin = strings.ReplaceAll(strings.TrimSpace(pin), " ", "")
	if pin == "" {
		return location{}, ErrPincodeRequired
	}
	if !pincodePattern.MatchString(pin) {
		return location{}, ErrPincodeInvalid
	}
	table, err := loadPincodes()
	if err != nil {
		return location{}, err
	}
	loc, ok := table[pin]
	if !ok {
		return location{}, ErrPincodeNotFound
	}
	return loc, nil
}

func cleanText(value string, maxRunes int, tooLong error) (string, error) {
	if !utf8.ValidString(value) {
		return "", ErrTextInvalid
	}
	for _, r := range value {
		if unicode.IsControl(r) && !unicode.IsSpace(r) {
			return "", ErrTextInvalid
		}
	}
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) > maxRunes {
		return "", tooLong
	}
	return value, nil
}

func normalizePhoneNumber(phoneNumber string) (string, error) {
	if strings.TrimSpace(phoneNumber) == "" {
		return "", ErrPhoneNumberRequired
	}
	parsed, err := utils.ValidatePhoneNumber(phoneNumber)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPhoneNumberInvalid, err)
	}
	return "+" + strconv.FormatUint(uint64(parsed.CountryCode), 10) + strconv.FormatUint(parsed.Number, 10), nil
}

func cloneMetadata(metadata map[string]any) (map[string]any, error) {
	if len(metadata) == 0 {
		return map[string]any{}, nil
	}
	raw, err := utils.FromObjectToStringJSON(metadata)
	if err != nil {
		return nil, err
	}
	if len(raw) > maxMetadataBytes {
		return nil, ErrMetadataTooLarge
	}
	return utils.FromStringToObjectJSON[map[string]any](raw)
}

func normalize(m Model, previous *Model) (Model, error) {
	var err error
	if m.FullName, err = cleanText(m.FullName, maxFullNameRunes, ErrFullNameTooLong); err != nil {
		return Model{}, err
	}
	if m.FullName == "" {
		return Model{}, ErrFullNameRequired
	}
	if m.PhoneNumber, err = normalizePhoneNumber(m.PhoneNumber); err != nil {
		return Model{}, err
	}
	if m.Label, err = cleanText(m.Label, maxLabelRunes, ErrLabelTooLong); err != nil {
		return Model{}, err
	}
	if m.AddressLine1, err = cleanText(m.AddressLine1, maxLineRunes, ErrAddressLine1TooLong); err != nil {
		return Model{}, err
	}
	if m.AddressLine1 == "" {
		return Model{}, ErrAddressLine1Required
	}
	if m.AddressLine2, err = cleanText(m.AddressLine2, maxLineRunes, ErrAddressLine2TooLong); err != nil {
		return Model{}, err
	}
	if m.Landmark, err = cleanText(m.Landmark, maxLandmarkRunes, ErrLandmarkTooLong); err != nil {
		return Model{}, err
	}
	if m.City, err = cleanText(m.City, maxCityRunes, ErrCityTooLong); err != nil {
		return Model{}, err
	}
	if _, ok := countryAliases[strings.ToLower(strings.TrimSpace(m.Country))]; !ok {
		return Model{}, ErrCountryUnsupported
	}
	loc, err := lookupPincode(m.Pincode)
	if err != nil {
		return Model{}, err
	}
	pincodeChanged := previous == nil || previous.Pincode != loc.pincode
	cityUntouched := previous != nil && m.City == previous.City
	if m.City == "" || (pincodeChanged && cityUntouched) {
		m.City = loc.city
	}
	m.Pincode = loc.pincode
	m.State = loc.state
	m.StateCode = loc.stateCode
	m.Country = countryIndia
	m.IsIndian = true
	if m.Metadata, err = cloneMetadata(m.Metadata); err != nil {
		return Model{}, err
	}
	return m, nil
}

func getAddress(tx *bolt.Tx, userID, addressID string) (Model, error) {
	if tx.Bucket(bucketUserIndex).Get(userIndexKey(userID, addressID)) == nil {
		return Model{}, ErrAddressNotFound
	}
	raw := tx.Bucket(bucketAddresses).Get([]byte(addressID))
	if raw == nil {
		return Model{}, ErrIndexCorrupted
	}
	address, err := utils.FromStringToObjectJSON[Model](string(raw))
	if err != nil {
		return Model{}, err
	}
	if address.UserID != userID || address.ID != addressID {
		return Model{}, ErrIndexCorrupted
	}
	if address.Metadata == nil {
		address.Metadata = map[string]any{}
	}
	address.IsDefault = string(tx.Bucket(bucketDefaults).Get([]byte(userID))) == addressID
	return address, nil
}

func putAddress(tx *bolt.Tx, address Model) error {
	address.IsDefault = false
	raw, err := utils.FromObjectToStringJSON(address)
	if err != nil {
		return err
	}
	if err := tx.Bucket(bucketAddresses).Put([]byte(address.ID), []byte(raw)); err != nil {
		return err
	}
	return tx.Bucket(bucketUserIndex).Put(userIndexKey(address.UserID, address.ID), []byte{})
}

func deleteAddress(tx *bolt.Tx, userID, addressID string) error {
	if err := tx.Bucket(bucketAddresses).Delete([]byte(addressID)); err != nil {
		return err
	}
	return tx.Bucket(bucketUserIndex).Delete(userIndexKey(userID, addressID))
}

func addressIDsOf(tx *bolt.Tx, userID string) []string {
	prefix := []byte(userID)
	ids := make([]string, 0)
	c := tx.Bucket(bucketUserIndex).Cursor()
	for k, _ := c.Seek(prefix); k != nil && strings.HasPrefix(string(k), userID); k, _ = c.Next() {
		ids = append(ids, string(k[len(prefix):]))
	}
	return ids
}

func listAddresses(tx *bolt.Tx, userID string) ([]Model, error) {
	ids := addressIDsOf(tx, userID)
	addresses := make([]Model, 0, len(ids))
	for _, id := range ids {
		address, err := getAddress(tx, userID, id)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}
	sort.SliceStable(addresses, func(i, j int) bool {
		if addresses[i].IsDefault != addresses[j].IsDefault {
			return addresses[i].IsDefault
		}
		return addresses[i].ID > addresses[j].ID
	})
	return addresses, nil
}

func setDefault(tx *bolt.Tx, userID, addressID string) error {
	return tx.Bucket(bucketDefaults).Put([]byte(userID), []byte(addressID))
}

func promoteNewest(tx *bolt.Tx, userID string) error {
	ids := addressIDsOf(tx, userID)
	if len(ids) == 0 {
		return tx.Bucket(bucketDefaults).Delete([]byte(userID))
	}
	return setDefault(tx, userID, ids[len(ids)-1])
}

func runMutate(mutate func(Model) (Model, error), current Model) (next Model, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			next = Model{}
			err = fmt.Errorf("%w: %v", ErrMutatePanicked, recovered)
		}
	}()
	snapshot := current
	snapshot.Metadata, err = cloneMetadata(current.Metadata)
	if err != nil {
		return Model{}, err
	}
	return mutate(snapshot)
}

func validateIDs(userID, addressID string) error {
	if !utils.ValidateULID(userID) {
		return ErrUserIDInvalid
	}
	if !utils.ValidateULID(addressID) {
		return ErrAddressIDInvalid
	}
	return nil
}
