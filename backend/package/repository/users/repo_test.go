package usersRepository_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	usersRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/users"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func newRepo(t *testing.T) usersRepository.Repository {
	t.Helper()
	repo, err := usersRepository.NewRepository([]string{t.TempDir(), "users.db"}, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

const testPassword = "Secr3t!pass"

var contactSeq atomic.Uint32

func nextPhone() string {
	return fmt.Sprintf("+9198000%05d", contactSeq.Add(1))
}

func nextEmail() string {
	return fmt.Sprintf("user%d@x.com", contactSeq.Add(1))
}

func mustCreate(t *testing.T, repo usersRepository.Repository, user usersRepository.Model) usersRepository.Model {
	t.Helper()
	if user.Password == "" {
		user.Password = testPassword
	}
	if user.Email == "" {
		user.Email = nextEmail()
	}
	if user.PhoneNumber == "" {
		user.PhoneNumber = nextPhone()
	}
	created, err := repo.CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return created
}

func TestCreateUser_NormalizesAndSetsServerFields(t *testing.T) {
	repo := newRepo(t)

	created := mustCreate(t, repo, usersRepository.Model{
		ID:                     "client-supplied",
		Name:                   "  Sahib  ",
		Email:                  "  Sahib@Example.COM ",
		PhoneNumber:            "98765 43210",
		IsEmailValidated:       true,
		IsPhoneNumberValidated: true,
	})

	if !utils.ValidateULID(created.ID) {
		t.Errorf("ID %q is not a ULID", created.ID)
	}
	if created.Name != "Sahib" {
		t.Errorf("Name = %q, want %q", created.Name, "Sahib")
	}
	if created.Email != "sahib@example.com" {
		t.Errorf("Email = %q, want %q", created.Email, "sahib@example.com")
	}
	if created.PhoneNumber != "+919876543210" {
		t.Errorf("PhoneNumber = %q, want %q", created.PhoneNumber, "+919876543210")
	}
	if created.IsEmailValidated || created.IsPhoneNumberValidated {
		t.Error("validation flags must start false")
	}
	if created.Metadata == nil {
		t.Error("Metadata must be non-nil")
	}
	if created.CreatedAt.IsZero() || !created.CreatedAt.Equal(created.UpdatedAt) {
		t.Errorf("CreatedAt %v / UpdatedAt %v not set equal", created.CreatedAt, created.UpdatedAt)
	}
	now, err := utils.TimeNow()
	if err != nil {
		t.Fatalf("TimeNow: %v", err)
	}
	if created.CreatedAt.Location().String() != now.Location().String() {
		t.Errorf("CreatedAt location = %v, want %v", created.CreatedAt.Location(), now.Location())
	}
}

func TestCreateUser_Errors(t *testing.T) {
	repo := newRepo(t)
	mustCreate(t, repo, usersRepository.Model{Name: "Taken", Email: "taken@x.com", PhoneNumber: "+919876543210"})

	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		user    usersRepository.Model
		wantErr error
	}{
		{"nil ctx", nil, usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "+919811111111"}, usersRepository.ErrCtxInvalid},
		{"canceled ctx", canceled, usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "+919811111111"}, context.Canceled},
		{"missing name", context.Background(), usersRepository.Model{Name: "   ", Email: "a@x.com", PhoneNumber: "+919811111111"}, usersRepository.ErrNameRequired},
		{"missing email", context.Background(), usersRepository.Model{Name: "A", PhoneNumber: "+919811111111"}, usersRepository.ErrEmailRequired},
		{"blank email", context.Background(), usersRepository.Model{Name: "A", Email: "  ", PhoneNumber: "+919811111111"}, usersRepository.ErrEmailRequired},
		{"missing phone", context.Background(), usersRepository.Model{Name: "A", Email: "a@x.com"}, usersRepository.ErrPhoneNumberRequired},
		{"blank phone", context.Background(), usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "   "}, usersRepository.ErrPhoneNumberRequired},
		{"missing both", context.Background(), usersRepository.Model{Name: "A"}, usersRepository.ErrEmailRequired},
		{"invalid email", context.Background(), usersRepository.Model{Name: "A", Email: "nope", PhoneNumber: "+919811111111"}, usersRepository.ErrEmailInvalid},
		{"invalid phone", context.Background(), usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "12345"}, usersRepository.ErrPhoneNumberInvalid},
		{"duplicate email different case", context.Background(), usersRepository.Model{Name: "A", Email: "TAKEN@x.com", PhoneNumber: "+919811111111"}, usersRepository.ErrEmailAlreadyExists},
		{"duplicate phone different format", context.Background(), usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "098765 43210"}, usersRepository.ErrPhoneNumberAlreadyExists},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.user.Password == "" {
				tt.user.Password = testPassword
			}
			got, err := repo.CreateUser(tt.ctx, tt.user)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got.ID != "" {
				t.Errorf("returned user %+v on error, want zero value", got)
			}
		})
	}
}

func TestCreateUser_FailedCreateLeavesNoIndexEntries(t *testing.T) {
	repo := newRepo(t)
	mustCreate(t, repo, usersRepository.Model{Name: "Taken", PhoneNumber: "+919876543210"})

	_, err := repo.CreateUser(context.Background(), usersRepository.Model{
		Password:    testPassword,
		Name:        "Loser",
		Email:       "fresh@x.com",
		PhoneNumber: "+919876543210",
	})
	if !errors.Is(err, usersRepository.ErrPhoneNumberAlreadyExists) {
		t.Fatalf("err = %v, want ErrPhoneNumberAlreadyExists", err)
	}

	if _, err := repo.GetUserByEmail(context.Background(), "fresh@x.com"); !errors.Is(err, usersRepository.ErrUserNotFound) {
		t.Errorf("email index leaked after rollback: err = %v", err)
	}
	mustCreate(t, repo, usersRepository.Model{Name: "Second try", Email: "fresh@x.com"})
}

func TestCreateUser_ConcurrentDuplicateEmail(t *testing.T) {
	repo := newRepo(t)

	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.CreateUser(context.Background(), usersRepository.Model{Name: "Race", Email: "race@x.com", PhoneNumber: nextPhone(), Password: testPassword})
			if err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			} else if !errors.Is(err, usersRepository.ErrEmailAlreadyExists) {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Errorf("successes = %d, want exactly 1", successes)
	}
}

func TestGetUserByID(t *testing.T) {
	repo := newRepo(t)
	created := mustCreate(t, repo, usersRepository.Model{
		Name:     "A",
		Email:    "a@x.com",
		Metadata: map[string]any{"plan": "gold"},
	})

	got, err := repo.GetUserByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.ID != created.ID || got.Email != created.Email || got.Name != created.Name {
		t.Errorf("got %+v, want %+v", got, created)
	}
	if !got.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, created.CreatedAt)
	}
	if got.Metadata["plan"] != "gold" {
		t.Errorf("Metadata = %v, want plan=gold", got.Metadata)
	}

	if _, err := repo.GetUserByID(context.Background(), "not-a-ulid"); !errors.Is(err, usersRepository.ErrIDInvalid) {
		t.Errorf("invalid id err = %v, want ErrIDInvalid", err)
	}
	if _, err := repo.GetUserByID(context.Background(), utils.NewULID()); !errors.Is(err, usersRepository.ErrUserNotFound) {
		t.Errorf("unknown id err = %v, want ErrUserNotFound", err)
	}
	if _, err := repo.GetUserByID(nil, created.ID); !errors.Is(err, usersRepository.ErrCtxInvalid) {
		t.Errorf("nil ctx err = %v, want ErrCtxInvalid", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	repo := newRepo(t)
	created := mustCreate(t, repo, usersRepository.Model{Name: "A", Email: "a@x.com"})

	for _, input := range []string{"a@x.com", "A@X.COM", "  a@x.com  ", "Alice <a@x.com>"} {
		got, err := repo.GetUserByEmail(context.Background(), input)
		if err != nil {
			t.Errorf("GetUserByEmail(%q): %v", input, err)
			continue
		}
		if got.ID != created.ID {
			t.Errorf("GetUserByEmail(%q).ID = %q, want %q", input, got.ID, created.ID)
		}
	}

	if _, err := repo.GetUserByEmail(context.Background(), "b@x.com"); !errors.Is(err, usersRepository.ErrUserNotFound) {
		t.Errorf("unknown email err = %v, want ErrUserNotFound", err)
	}
	if _, err := repo.GetUserByEmail(context.Background(), "nope"); !errors.Is(err, usersRepository.ErrEmailInvalid) {
		t.Errorf("invalid email err = %v, want ErrEmailInvalid", err)
	}
}

func TestGetUserByPhoneNumber(t *testing.T) {
	repo := newRepo(t)
	created := mustCreate(t, repo, usersRepository.Model{Name: "A", PhoneNumber: "+919876543210"})

	for _, input := range []string{"+919876543210", "9876543210", "+91 98765 43210", "098765-43210"} {
		got, err := repo.GetUserByPhoneNumber(context.Background(), input)
		if err != nil {
			t.Errorf("GetUserByPhoneNumber(%q): %v", input, err)
			continue
		}
		if got.ID != created.ID {
			t.Errorf("GetUserByPhoneNumber(%q).ID = %q, want %q", input, got.ID, created.ID)
		}
	}

	if _, err := repo.GetUserByPhoneNumber(context.Background(), "+16502530000"); !errors.Is(err, usersRepository.ErrUserNotFound) {
		t.Errorf("unknown phone err = %v, want ErrUserNotFound", err)
	}
	if _, err := repo.GetUserByPhoneNumber(context.Background(), "123"); !errors.Is(err, usersRepository.ErrPhoneNumberInvalid) {
		t.Errorf("invalid phone err = %v, want ErrPhoneNumberInvalid", err)
	}
}

func TestValidateEmail(t *testing.T) {
	repo := newRepo(t)
	withEmail := mustCreate(t, repo, usersRepository.Model{Name: "A", Email: "a@x.com"})

	got, err := repo.ValidateEmail(context.Background(), withEmail.ID)
	if err != nil {
		t.Fatalf("ValidateEmail: %v", err)
	}
	if !got.IsEmailValidated {
		t.Error("IsEmailValidated = false, want true")
	}
	if got.IsPhoneNumberValidated {
		t.Error("IsPhoneNumberValidated changed, want false")
	}
	if !got.UpdatedAt.After(withEmail.UpdatedAt) {
		t.Errorf("UpdatedAt %v not after %v", got.UpdatedAt, withEmail.UpdatedAt)
	}

	stored, _ := repo.GetUserByID(context.Background(), withEmail.ID)
	if !stored.IsEmailValidated {
		t.Error("IsEmailValidated not persisted")
	}

	if _, err := repo.ValidateEmail(context.Background(), withEmail.ID); err != nil {
		t.Errorf("validating twice: %v", err)
	}
	if _, err := repo.ValidateEmail(context.Background(), utils.NewULID()); !errors.Is(err, usersRepository.ErrUserNotFound) {
		t.Errorf("unknown id err = %v, want ErrUserNotFound", err)
	}
	if _, err := repo.ValidateEmail(context.Background(), "bad"); !errors.Is(err, usersRepository.ErrIDInvalid) {
		t.Errorf("bad id err = %v, want ErrIDInvalid", err)
	}
}

func TestValidatePhoneNumber(t *testing.T) {
	repo := newRepo(t)
	withPhone := mustCreate(t, repo, usersRepository.Model{Name: "A", PhoneNumber: "+919876543210"})

	got, err := repo.ValidatePhoneNumber(context.Background(), withPhone.ID)
	if err != nil {
		t.Fatalf("ValidatePhoneNumber: %v", err)
	}
	if !got.IsPhoneNumberValidated {
		t.Error("IsPhoneNumberValidated = false, want true")
	}
	if got.IsEmailValidated {
		t.Error("IsEmailValidated changed, want false")
	}

	stored, _ := repo.GetUserByPhoneNumber(context.Background(), "9876543210")
	if !stored.IsPhoneNumberValidated {
		t.Error("IsPhoneNumberValidated not persisted")
	}

	if _, err := repo.ValidatePhoneNumber(context.Background(), utils.NewULID()); !errors.Is(err, usersRepository.ErrUserNotFound) {
		t.Errorf("unknown id err = %v, want ErrUserNotFound", err)
	}
}

func TestUpdateMetadata(t *testing.T) {
	repo := newRepo(t)
	created := mustCreate(t, repo, usersRepository.Model{
		Name:     "A",
		Email:    "a@x.com",
		Metadata: map[string]any{"plan": "free", "locale": "en"},
	})

	got, err := repo.UpdateMetadata(context.Background(), created.ID, map[string]any{"plan": "gold", "ref": "x1"})
	if err != nil {
		t.Fatalf("UpdateMetadata: %v", err)
	}
	want := map[string]any{"plan": "gold", "locale": "en", "ref": "x1"}
	for k, v := range want {
		if got.Metadata[k] != v {
			t.Errorf("Metadata[%q] = %v, want %v", k, got.Metadata[k], v)
		}
	}
	if len(got.Metadata) != len(want) {
		t.Errorf("Metadata = %v, want %v", got.Metadata, want)
	}

	stored, _ := repo.GetUserByID(context.Background(), created.ID)
	if stored.Metadata["plan"] != "gold" || stored.Metadata["locale"] != "en" {
		t.Errorf("stored Metadata = %v, want merged", stored.Metadata)
	}
	if stored.Email != created.Email || stored.IsEmailValidated {
		t.Errorf("other fields changed: %+v", stored)
	}

	if _, err := repo.UpdateMetadata(context.Background(), created.ID, nil); err != nil {
		t.Errorf("nil metadata: %v", err)
	}
	if _, err := repo.UpdateMetadata(context.Background(), utils.NewULID(), map[string]any{"a": 1}); !errors.Is(err, usersRepository.ErrUserNotFound) {
		t.Errorf("unknown id err = %v, want ErrUserNotFound", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := repo.UpdateMetadata(canceled, created.ID, map[string]any{"a": 1}); !errors.Is(err, usersRepository.ErrCtxInvalid) {
		t.Errorf("canceled ctx err = %v, want ErrCtxInvalid", err)
	}
}

func TestRepository_PersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	path := []string{dir, "users.db"}

	repo, err := usersRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	created := mustCreate(t, repo, usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "+919876543210"})
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := usersRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	if got, err := reopened.GetUserByEmail(context.Background(), "a@x.com"); err != nil || got.ID != created.ID {
		t.Errorf("by email after reopen = %+v, %v", got, err)
	}
	if got, err := reopened.GetUserByPhoneNumber(context.Background(), "9876543210"); err != nil || got.ID != created.ID {
		t.Errorf("by phone after reopen = %+v, %v", got, err)
	}
}

func TestNewRepository_LockedFileTimesOut(t *testing.T) {
	path := []string{t.TempDir(), "users.db"}
	first, err := usersRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	defer first.Close()

	if _, err := usersRepository.NewRepository(path, 0o600); err == nil {
		t.Error("second open of a locked file succeeded, want timeout error")
	}
}

func TestCreateUser_PasswordIsNeverReturnedOrStoredInPlainText(t *testing.T) {
	dir := t.TempDir()
	path := []string{dir, "users.db"}
	repo, err := usersRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	const secret = "UniquePlain!42xyz"
	created, err := repo.CreateUser(context.Background(), usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "+919876543210", Password: secret})
	if err != nil {
		t.Fatal(err)
	}
	if created.Password != "" {
		t.Errorf("CreateUser returned the password %q", created.Password)
	}
	got, _ := repo.GetUserByID(context.Background(), created.ID)
	if got.Password != "" {
		t.Errorf("GetUserByID returned the password %q", got.Password)
	}
	asJSON, err := utils.FromObjectToStringJSON(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(asJSON, "password") {
		t.Errorf("Model JSON exposes a password field: %s", asJSON)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(path...))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte(secret)) {
		t.Error("plain-text password found in the database file")
	}
	if !bytes.Contains(raw, []byte(`"password_hash"`)) || !bytes.Contains(raw, []byte(`"password_salt"`)) {
		t.Error("password hash and salt not found in the stored record")
	}
}

func TestCreateUser_RejectsWeakPassword(t *testing.T) {
	repo := newRepo(t)
	tests := []struct {
		name     string
		password string
		cause    error
	}{
		{"empty", "", utils.ErrPasswordTooShort},
		{"too short", "a1!", utils.ErrPasswordTooShort},
		{"too long", strings.Repeat("a", 64) + "1!", utils.ErrPasswordTooLong},
		{"no special", "abcdefg1", utils.ErrPasswordTooWeak},
		{"control character", "abc\tdef1!", utils.ErrPasswordInvalidCharacters},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := repo.CreateUser(context.Background(), usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "+919876543210", Password: tt.password})
			if !errors.Is(err, usersRepository.ErrPasswordInvalid) || !errors.Is(err, tt.cause) {
				t.Errorf("err = %v, want ErrPasswordInvalid wrapping %v", err, tt.cause)
			}
		})
	}
	if _, err := repo.GetUserByEmail(context.Background(), "a@x.com"); !errors.Is(err, usersRepository.ErrUserNotFound) {
		t.Errorf("rejected user was stored: %v", err)
	}
}

func TestGetUserByEmailAndPassword(t *testing.T) {
	repo := newRepo(t)
	created := mustCreate(t, repo, usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "+919876543210"})
	mustCreate(t, repo, usersRepository.Model{Name: "B", Email: "b@x.com", Password: "Other!pass9"})

	for _, email := range []string{"a@x.com", "  A@X.COM ", "Alice <a@x.com>"} {
		got, err := repo.GetUserByEmailAndPassword(context.Background(), email, testPassword)
		if err != nil {
			t.Errorf("GetUserByEmailAndPassword(%q): %v", email, err)
			continue
		}
		if got.ID != created.ID || got.Password != "" {
			t.Errorf("got ID %q password %q, want %q and empty", got.ID, got.Password, created.ID)
		}
	}

	tests := []struct {
		name, email, password string
	}{
		{"wrong password", "a@x.com", "Wrong!pass1"},
		{"another user's password", "a@x.com", "Other!pass9"},
		{"password with extra space", "a@x.com", testPassword + " "},
		{"password case changed", "a@x.com", strings.ToUpper(testPassword)},
		{"empty password", "a@x.com", ""},
		{"unknown email", "nobody@x.com", testPassword},
		{"malformed email", "nope", testPassword},
		{"empty email", "", testPassword},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.GetUserByEmailAndPassword(context.Background(), tt.email, tt.password)
			if !errors.Is(err, usersRepository.ErrInvalidCredentials) {
				t.Errorf("err = %v, want ErrInvalidCredentials", err)
			}
			if got.ID != "" {
				t.Errorf("user returned on failed login: %+v", got)
			}
		})
	}

	if _, err := repo.GetUserByEmailAndPassword(nil, "a@x.com", testPassword); !errors.Is(err, usersRepository.ErrCtxInvalid) {
		t.Errorf("nil ctx err = %v", err)
	}
}

func TestGetUserByPhoneNumberAndPassword(t *testing.T) {
	repo := newRepo(t)
	created := mustCreate(t, repo, usersRepository.Model{Name: "A", PhoneNumber: "+919876543210"})
	mustCreate(t, repo, usersRepository.Model{Name: "B", Email: "b@x.com"})

	for _, phone := range []string{"+919876543210", "98765 43210", "098765-43210"} {
		got, err := repo.GetUserByPhoneNumberAndPassword(context.Background(), phone, testPassword)
		if err != nil || got.ID != created.ID {
			t.Errorf("GetUserByPhoneNumberAndPassword(%q) = %q, %v", phone, got.ID, err)
		}
	}
	for name, tc := range map[string][2]string{
		"wrong password":  {"+919876543210", "Wrong!pass1"},
		"unknown phone":   {"+919876543211", testPassword},
		"malformed phone": {"123", testPassword},
	} {
		if _, err := repo.GetUserByPhoneNumberAndPassword(context.Background(), tc[0], tc[1]); !errors.Is(err, usersRepository.ErrInvalidCredentials) {
			t.Errorf("%s: err = %v, want ErrInvalidCredentials", name, err)
		}
	}
}

func TestPasswordSurvivesUpdatesAndReopen(t *testing.T) {
	path := []string{t.TempDir(), "users.db"}
	repo, err := usersRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	created := mustCreate(t, repo, usersRepository.Model{Name: "A", Email: "a@x.com", PhoneNumber: "+919876543210"})
	ctx := context.Background()
	if _, err := repo.ValidateEmail(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ValidatePhoneNumber(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateMetadata(ctx, created.ID, map[string]any{"plan": "gold"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetUserByEmailAndPassword(ctx, "a@x.com", testPassword); err != nil {
		t.Errorf("password lost after updates: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := usersRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.GetUserByEmailAndPassword(ctx, "a@x.com", testPassword)
	if err != nil {
		t.Fatalf("login after reopen: %v", err)
	}
	if !got.IsEmailValidated || !got.IsPhoneNumberValidated || got.Metadata["plan"] != "gold" {
		t.Errorf("fields lost: %+v", got)
	}
}
