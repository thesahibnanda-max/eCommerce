package addressRepository

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var bg = context.Background()

func newTestRepo(t *testing.T) *repository {
	t.Helper()
	r, err := NewRepository([]string{t.TempDir(), "address.db"}, 0o600)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r.(*repository)
}

func validAddress(userID string) Model {
	return Model{
		UserID:       userID,
		Label:        "  Home ",
		FullName:     "  Riya   Sharma ",
		PhoneNumber:  "+91 98765 43210",
		AddressLine1: " 12, MG   Road ",
		AddressLine2: "Flat 4B",
		Landmark:     "near metro",
		Pincode:      "110 001",
		Metadata:     map[string]any{"gate": "2"},
	}
}

func mustCreate(t *testing.T, r *repository, m Model) Model {
	t.Helper()
	created, err := r.CreateAddress(bg, m)
	if err != nil {
		t.Fatalf("CreateAddress() error = %v", err)
	}
	return created
}

func TestCreateAddress_NormalizesAndDefaults(t *testing.T) {
	r := newTestRepo(t)
	stamp := time.Unix(1_700_000_000, 0)
	r.now = func() (time.Time, error) { return stamp, nil }
	user := utils.NewULID()
	got := mustCreate(t, r, validAddress(user))
	if !utils.ValidateULID(got.ID) || got.UserID != user {
		t.Fatalf("ids = %q %q", got.ID, got.UserID)
	}
	if got.FullName != "Riya Sharma" || got.Label != "Home" || got.AddressLine1 != "12, MG Road" {
		t.Fatalf("text not cleaned: %+v", got)
	}
	if got.PhoneNumber != "+919876543210" {
		t.Fatalf("PhoneNumber = %q", got.PhoneNumber)
	}
	if got.Pincode != "110001" || got.City != "New Delhi" || got.State != "Delhi" || got.StateCode != "IN-DL" || got.Country != "India" || !got.IsIndian {
		t.Fatalf("location = %+v", got)
	}
	if !got.IsDefault || !got.CreatedAt.Equal(stamp) || !got.UpdatedAt.Equal(stamp) {
		t.Fatalf("defaults = %+v", got)
	}
	stored, err := r.GetAddress(bg, user, got.ID)
	if err != nil {
		t.Fatalf("GetAddress() error = %v", err)
	}
	if stored.FullName != got.FullName || !stored.IsDefault || stored.Metadata["gate"] != "2" {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestCreateAddress_KeepsCustomCity(t *testing.T) {
	r := newTestRepo(t)
	m := validAddress(utils.NewULID())
	m.City = "Connaught Place"
	if got := mustCreate(t, r, m); got.City != "Connaught Place" || got.State != "Delhi" {
		t.Fatalf("got = %+v", got)
	}
}

func TestCreateAddress_IgnoresCallerIDsAndCopiesMetadata(t *testing.T) {
	r := newTestRepo(t)
	m := validAddress(utils.NewULID())
	m.ID = "custom"
	got := mustCreate(t, r, m)
	if got.ID == "custom" {
		t.Fatal("caller supplied ID was kept")
	}
	m.Metadata["gate"] = "9"
	if got.Metadata["gate"] != "2" {
		t.Fatal("metadata shares caller map")
	}
}

func TestCreateAddress_Validation(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	cases := []struct {
		name   string
		mutate func(*Model)
		want   error
	}{
		{"user id", func(m *Model) { m.UserID = "nope" }, ErrUserIDInvalid},
		{"name missing", func(m *Model) { m.FullName = "   " }, ErrFullNameRequired},
		{"name long", func(m *Model) { m.FullName = strings.Repeat("a", maxFullNameRunes+1) }, ErrFullNameTooLong},
		{"phone missing", func(m *Model) { m.PhoneNumber = "" }, ErrPhoneNumberRequired},
		{"phone invalid", func(m *Model) { m.PhoneNumber = "12345" }, ErrPhoneNumberInvalid},
		{"line1 missing", func(m *Model) { m.AddressLine1 = " " }, ErrAddressLine1Required},
		{"line1 long", func(m *Model) { m.AddressLine1 = strings.Repeat("x", maxLineRunes+1) }, ErrAddressLine1TooLong},
		{"line2 long", func(m *Model) { m.AddressLine2 = strings.Repeat("x", maxLineRunes+1) }, ErrAddressLine2TooLong},
		{"landmark long", func(m *Model) { m.Landmark = strings.Repeat("x", maxLandmarkRunes+1) }, ErrLandmarkTooLong},
		{"city long", func(m *Model) { m.City = strings.Repeat("x", maxCityRunes+1) }, ErrCityTooLong},
		{"label long", func(m *Model) { m.Label = strings.Repeat("x", maxLabelRunes+1) }, ErrLabelTooLong},
		{"control char", func(m *Model) { m.AddressLine1 = "12\x00Road" }, ErrTextInvalid},
		{"bad utf8", func(m *Model) { m.FullName = "\xff" }, ErrTextInvalid},
		{"pincode missing", func(m *Model) { m.Pincode = "" }, ErrPincodeRequired},
		{"pincode invalid", func(m *Model) { m.Pincode = "12ab56" }, ErrPincodeInvalid},
		{"pincode leading zero", func(m *Model) { m.Pincode = "010001" }, ErrPincodeInvalid},
		{"pincode unknown", func(m *Model) { m.Pincode = "999999" }, ErrPincodeNotFound},
		{"country", func(m *Model) { m.Country = "Nepal" }, ErrCountryUnsupported},
		{"metadata big", func(m *Model) { m.Metadata = map[string]any{"x": strings.Repeat("y", maxMetadataBytes)} }, ErrMetadataTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validAddress(user)
			tc.mutate(&m)
			if _, err := r.CreateAddress(bg, m); !errors.Is(err, tc.want) {
				t.Fatalf("CreateAddress() error = %v, want %v", err, tc.want)
			}
		})
	}
	if n, _ := r.CountAddresses(bg, user); n != 0 {
		t.Fatalf("failed creates stored %d addresses", n)
	}
}

func TestCreateAddress_CountryAliases(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	for _, c := range []string{"", "india", " IN ", "Bharat"} {
		m := validAddress(user)
		m.Country = c
		if got := mustCreate(t, r, m); got.Country != "India" {
			t.Fatalf("country %q -> %q", c, got.Country)
		}
	}
}

func TestCreateAddress_Limit(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	for i := 0; i < maxAddressesPerUser; i++ {
		mustCreate(t, r, validAddress(user))
	}
	if _, err := r.CreateAddress(bg, validAddress(user)); !errors.Is(err, ErrAddressLimitReached) {
		t.Fatalf("error = %v, want ErrAddressLimitReached", err)
	}
	mustCreate(t, r, validAddress(utils.NewULID()))
}

func TestCreateAddress_ExplicitDefaultMovesDefault(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	first := mustCreate(t, r, validAddress(user))
	second := mustCreate(t, r, validAddress(user))
	if second.IsDefault {
		t.Fatal("second address became default without asking")
	}
	m := validAddress(user)
	m.IsDefault = true
	third := mustCreate(t, r, m)
	if !third.IsDefault {
		t.Fatal("explicit default ignored")
	}
	got, err := r.GetAddress(bg, user, first.ID)
	if err != nil || got.IsDefault {
		t.Fatalf("first still default: %+v %v", got, err)
	}
	def, err := r.GetDefaultAddress(bg, user)
	if err != nil || def.ID != third.ID {
		t.Fatalf("GetDefaultAddress() = %+v %v", def, err)
	}
}

func TestGetAddress_ScopedToOwner(t *testing.T) {
	r := newTestRepo(t)
	owner := utils.NewULID()
	created := mustCreate(t, r, validAddress(owner))
	if _, err := r.GetAddress(bg, utils.NewULID(), created.ID); !errors.Is(err, ErrAddressNotFound) {
		t.Fatalf("other user error = %v", err)
	}
	if _, err := r.GetAddress(bg, owner, utils.NewULID()); !errors.Is(err, ErrAddressNotFound) {
		t.Fatalf("missing error = %v", err)
	}
	if _, err := r.GetAddress(bg, owner, "bad"); !errors.Is(err, ErrAddressIDInvalid) {
		t.Fatalf("bad id error = %v", err)
	}
	if _, err := r.GetAddress(bg, "bad", created.ID); !errors.Is(err, ErrUserIDInvalid) {
		t.Fatalf("bad user error = %v", err)
	}
}

func TestListAddresses_DefaultFirstThenNewest(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	a := mustCreate(t, r, validAddress(user))
	b := mustCreate(t, r, validAddress(user))
	c := mustCreate(t, r, validAddress(user))
	mustCreate(t, r, validAddress(utils.NewULID()))
	list, err := r.ListAddresses(bg, user)
	if err != nil {
		t.Fatalf("ListAddresses() error = %v", err)
	}
	if len(list) != 3 || list[0].ID != a.ID || list[1].ID != c.ID || list[2].ID != b.ID {
		t.Fatalf("order = %v", ids(list))
	}
	empty, err := r.ListAddresses(bg, utils.NewULID())
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list = %v %v", empty, err)
	}
	if n, err := r.CountAddresses(bg, user); err != nil || n != 3 {
		t.Fatalf("CountAddresses() = %d %v", n, err)
	}
}

func TestGetDefaultAddress_None(t *testing.T) {
	r := newTestRepo(t)
	if _, err := r.GetDefaultAddress(bg, utils.NewULID()); !errors.Is(err, ErrNoDefaultAddress) {
		t.Fatalf("error = %v", err)
	}
}

func TestUpdateAddress(t *testing.T) {
	r := newTestRepo(t)
	created := time.Unix(1_700_000_000, 0)
	updated := created.Add(time.Hour)
	r.now = func() (time.Time, error) { return created, nil }
	user := utils.NewULID()
	a := mustCreate(t, r, validAddress(user))
	r.now = func() (time.Time, error) { return updated, nil }
	got, err := r.UpdateAddress(bg, user, a.ID, func(m Model) (Model, error) {
		m.ID = utils.NewULID()
		m.UserID = utils.NewULID()
		m.CreatedAt = time.Time{}
		m.IsDefault = false
		m.Landmark = "  opposite   park "
		m.Metadata["floor"] = 3
		return m, nil
	})
	if err != nil {
		t.Fatalf("UpdateAddress() error = %v", err)
	}
	if got.ID != a.ID || got.UserID != user || !got.CreatedAt.Equal(created) || !got.UpdatedAt.Equal(updated) || !got.IsDefault {
		t.Fatalf("protected fields changed: %+v", got)
	}
	if got.Landmark != "opposite park" {
		t.Fatalf("Landmark = %q", got.Landmark)
	}
	stored, _ := r.GetAddress(bg, user, a.ID)
	if stored.Landmark != "opposite park" || stored.Metadata["floor"] != float64(3) || stored.Metadata["gate"] != "2" {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestUpdateAddress_PincodeRefreshesAutoCity(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	a := mustCreate(t, r, validAddress(user))
	got, err := r.UpdateAddress(bg, user, a.ID, func(m Model) (Model, error) {
		m.Pincode = "121001"
		return m, nil
	})
	if err != nil {
		t.Fatalf("UpdateAddress() error = %v", err)
	}
	if got.City != "Faridabad" || got.State != "Haryana" || got.StateCode != "IN-HR" {
		t.Fatalf("location = %+v", got)
	}
	got, err = r.UpdateAddress(bg, user, a.ID, func(m Model) (Model, error) {
		m.Pincode = "110001"
		m.City = "Janpath"
		return m, nil
	})
	if err != nil || got.City != "Janpath" || got.State != "Delhi" {
		t.Fatalf("custom city = %+v %v", got, err)
	}
}

func TestUpdateAddress_Failures(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	a := mustCreate(t, r, validAddress(user))
	if _, err := r.UpdateAddress(bg, user, a.ID, nil); !errors.Is(err, ErrMutateRequired) {
		t.Fatalf("nil mutate = %v", err)
	}
	boom := errors.New("boom")
	if _, err := r.UpdateAddress(bg, user, a.ID, func(Model) (Model, error) { return Model{}, boom }); !errors.Is(err, boom) {
		t.Fatalf("mutate error = %v", err)
	}
	if _, err := r.UpdateAddress(bg, user, a.ID, func(Model) (Model, error) { panic("oops") }); !errors.Is(err, ErrMutatePanicked) {
		t.Fatalf("panic = %v", err)
	}
	if _, err := r.UpdateAddress(bg, user, a.ID, func(m Model) (Model, error) {
		m.FullName = ""
		return m, nil
	}); !errors.Is(err, ErrFullNameRequired) {
		t.Fatalf("invalid update = %v", err)
	}
	if _, err := r.UpdateAddress(bg, utils.NewULID(), a.ID, func(m Model) (Model, error) { return m, nil }); !errors.Is(err, ErrAddressNotFound) {
		t.Fatalf("other user = %v", err)
	}
	stored, _ := r.GetAddress(bg, user, a.ID)
	if stored.FullName != a.FullName || !stored.UpdatedAt.Equal(a.UpdatedAt) {
		t.Fatalf("failed updates changed record: %+v", stored)
	}
}

func TestSetDefaultAddress(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	a := mustCreate(t, r, validAddress(user))
	b := mustCreate(t, r, validAddress(user))
	got, err := r.SetDefaultAddress(bg, user, b.ID)
	if err != nil || !got.IsDefault || got.ID != b.ID {
		t.Fatalf("SetDefaultAddress() = %+v %v", got, err)
	}
	if old, _ := r.GetAddress(bg, user, a.ID); old.IsDefault {
		t.Fatal("old default kept flag")
	}
	if again, err := r.SetDefaultAddress(bg, user, b.ID); err != nil || !again.IsDefault {
		t.Fatalf("idempotent = %+v %v", again, err)
	}
	if _, err := r.SetDefaultAddress(bg, utils.NewULID(), b.ID); !errors.Is(err, ErrAddressNotFound) {
		t.Fatalf("other user = %v", err)
	}
}

func TestDeleteAddress_PromotesNewest(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	a := mustCreate(t, r, validAddress(user))
	b := mustCreate(t, r, validAddress(user))
	c := mustCreate(t, r, validAddress(user))
	if err := r.DeleteAddress(bg, utils.NewULID(), a.ID); !errors.Is(err, ErrAddressNotFound) {
		t.Fatalf("other user delete = %v", err)
	}
	if err := r.DeleteAddress(bg, user, a.ID); err != nil {
		t.Fatalf("DeleteAddress() error = %v", err)
	}
	if _, err := r.GetAddress(bg, user, a.ID); !errors.Is(err, ErrAddressNotFound) {
		t.Fatalf("deleted still readable: %v", err)
	}
	def, err := r.GetDefaultAddress(bg, user)
	if err != nil || def.ID != c.ID {
		t.Fatalf("promoted = %+v %v", def, err)
	}
	if err := r.DeleteAddress(bg, user, b.ID); err != nil {
		t.Fatalf("delete non-default = %v", err)
	}
	if def, _ := r.GetDefaultAddress(bg, user); def.ID != c.ID {
		t.Fatalf("default moved on non-default delete: %s", def.ID)
	}
	if err := r.DeleteAddress(bg, user, c.ID); err != nil {
		t.Fatalf("delete last = %v", err)
	}
	if _, err := r.GetDefaultAddress(bg, user); !errors.Is(err, ErrNoDefaultAddress) {
		t.Fatalf("default after last delete = %v", err)
	}
	if err := r.DeleteAddress(bg, user, c.ID); !errors.Is(err, ErrAddressNotFound) {
		t.Fatalf("double delete = %v", err)
	}
}

func TestDeleteAllAddresses(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	other := utils.NewULID()
	for i := 0; i < 3; i++ {
		mustCreate(t, r, validAddress(user))
	}
	kept := mustCreate(t, r, validAddress(other))
	n, err := r.DeleteAllAddresses(bg, user)
	if err != nil || n != 3 {
		t.Fatalf("DeleteAllAddresses() = %d %v", n, err)
	}
	if c, _ := r.CountAddresses(bg, user); c != 0 {
		t.Fatalf("left %d", c)
	}
	if _, err := r.GetDefaultAddress(bg, user); !errors.Is(err, ErrNoDefaultAddress) {
		t.Fatalf("default left: %v", err)
	}
	if def, err := r.GetDefaultAddress(bg, other); err != nil || def.ID != kept.ID {
		t.Fatalf("other user touched: %+v %v", def, err)
	}
	if first := mustCreate(t, r, validAddress(user)); !first.IsDefault {
		t.Fatal("first address after wipe is not default")
	}
}

func TestPincodeTable(t *testing.T) {
	table, err := loadPincodes()
	if err != nil {
		t.Fatalf("loadPincodes() error = %v", err)
	}
	if len(table) < 19000 {
		t.Fatalf("only %d pincodes loaded", len(table))
	}
	cases := map[string]location{
		"110001": {pincode: "110001", city: "New Delhi", state: "Delhi", stateCode: "IN-DL"},
		"560105": {pincode: "560105", city: "Bengaluru Urban", state: "Karnataka", stateCode: "IN-KA"},
		"700001": {pincode: "700001", city: "Kolkata", state: "West Bengal", stateCode: "IN-WB"},
	}
	for pin, want := range cases {
		if got, err := lookupPincode(pin); err != nil || got != want {
			t.Fatalf("lookupPincode(%s) = %+v %v", pin, got, err)
		}
	}
	for _, pin := range []string{"999999", "100001", "190000"} {
		if _, err := lookupPincode(pin); !errors.Is(err, ErrPincodeNotFound) {
			t.Fatalf("lookupPincode(%s) error = %v", pin, err)
		}
	}
}

func TestParsePincodes_Rejects(t *testing.T) {
	cases := map[string]string{
		"empty":     "",
		"no rows":   "pincode,city,state,state_code\n",
		"bad pin":   "pincode,city,state,state_code\n012345,A,B,IN-X\n",
		"blank":     "pincode,city,state,state_code\n110001,,Delhi,IN-DL\n",
		"columns":   "pincode,city,state,state_code\n110001,A,B\n",
		"duplicate": "pincode,city,state,state_code\n110001,A,B,IN-X\n110001,A,B,IN-X\n",
	}
	for name, raw := range cases {
		if _, err := parsePincodes(raw); !errors.Is(err, ErrPincodeLookupFailed) {
			t.Fatalf("%s: error = %v", name, err)
		}
	}
}

func TestContextChecked(t *testing.T) {
	r := newTestRepo(t)
	ctx, cancel := context.WithCancel(bg)
	cancel()
	user := utils.NewULID()
	id := utils.NewULID()
	mutate := func(m Model) (Model, error) { return m, nil }
	checks := []error{
		func() error { _, err := r.CreateAddress(ctx, validAddress(user)); return err }(),
		func() error { _, err := r.GetAddress(ctx, user, id); return err }(),
		func() error { _, err := r.ListAddresses(ctx, user); return err }(),
		func() error { _, err := r.CountAddresses(ctx, user); return err }(),
		func() error { _, err := r.GetDefaultAddress(ctx, user); return err }(),
		func() error { _, err := r.UpdateAddress(ctx, user, id, mutate); return err }(),
		func() error { _, err := r.SetDefaultAddress(ctx, user, id); return err }(),
		r.DeleteAddress(ctx, user, id),
		func() error { _, err := r.DeleteAllAddresses(ctx, user); return err }(),
	}
	for i, err := range checks {
		if err == nil {
			t.Fatalf("call %d accepted a cancelled context", i)
		}
	}
}

func TestCreateAddress_ConcurrentRespectsLimitAndSingleDefault(t *testing.T) {
	r := newTestRepo(t)
	user := utils.NewULID()
	var wg sync.WaitGroup
	for i := 0; i < maxAddressesPerUser+10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.CreateAddress(bg, validAddress(user))
		}()
	}
	wg.Wait()
	list, err := r.ListAddresses(bg, user)
	if err != nil || len(list) != maxAddressesPerUser {
		t.Fatalf("stored %d, err %v", len(list), err)
	}
	defaults := 0
	for _, a := range list {
		if a.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("defaults = %d", defaults)
	}
}

func ids(list []Model) []string {
	out := make([]string, len(list))
	for i, a := range list {
		out[i] = a.ID
	}
	return out
}
