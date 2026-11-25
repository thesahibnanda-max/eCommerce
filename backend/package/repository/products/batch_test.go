package productsRepository_test

import (
	"errors"
	"testing"

	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func totalProducts(t *testing.T, repo productsRepository.Repository) int {
	t.Helper()
	_, total, err := repo.ListProducts(bg, 0, 100)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	return total
}

func TestCreateProducts_SavesAllAndIndexes(t *testing.T) {
	repo := newRepo(t)
	created, err := repo.CreateProducts(bg, []Model{
		product("Red Shirt", []string{"shirts"}, variant("RS-1", 100, 1)),
		product("Blue Jeans", []string{"jeans"}, variant("BJ-1", 200, 1), variant("BJ-2", 250, 0)),
	})
	if err != nil {
		t.Fatalf("CreateProducts: %v", err)
	}
	if len(created) != 2 || created[0].Name != "Red Shirt" || created[1].Name != "Blue Jeans" {
		t.Fatalf("created = %v, want input order", names(created))
	}
	for _, p := range created {
		if !utils.ValidateULID(p.ID) {
			t.Errorf("product id %q is not a ULID", p.ID)
		}
		for _, v := range p.Variants {
			if !utils.ValidateULID(v.ID) {
				t.Errorf("variant id %q is not a ULID", v.ID)
			}
		}
	}
	if got := totalProducts(t, repo); got != 2 {
		t.Fatalf("total = %d, want 2", got)
	}
	if _, v, err := repo.GetProductBySKU(bg, "BJ-2"); err != nil || v.SKU != "BJ-2" {
		t.Fatalf("GetProductBySKU = %v, %v", v, err)
	}
	res, err := repo.SearchProducts(bg, productsRepository.SearchQuery{Text: "jeans", Limit: 10})
	if err != nil {
		t.Fatalf("SearchProducts: %v", err)
	}
	if res.Total != 1 || res.Products[0].ID != created[1].ID {
		t.Fatalf("search = %v, want Blue Jeans", names(res.Products))
	}
}

func TestCreateProducts_AllOrNothing(t *testing.T) {
	tests := []struct {
		name    string
		batch   []Model
		wantErr error
	}{
		{"empty batch", nil, productsRepository.ErrBatchEmpty},
		{"sku repeated across batch", []Model{
			product("A", nil, variant("DUP", 100, 1)),
			product("B", nil, variant("DUP", 100, 1)),
		}, productsRepository.ErrSKUDuplicateInBatch},
		{"sku already stored", []Model{
			product("A", nil, variant("NEW", 100, 1)),
			product("B", nil, variant("EXISTING", 100, 1)),
		}, productsRepository.ErrSKUAlreadyExists},
		{"one invalid item", []Model{
			product("A", nil, variant("OK", 100, 1)),
			product("", nil, variant("OK-2", 100, 1)),
		}, productsRepository.ErrNameRequired},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			mustCreate(t, repo, product("Existing", nil, variant("EXISTING", 100, 1)))
			got, err := repo.CreateProducts(bg, tc.batch)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got != nil {
				t.Fatalf("got %v, want nil", names(got))
			}
			if total := totalProducts(t, repo); total != 1 {
				t.Fatalf("total = %d, want 1 (nothing saved)", total)
			}
			if _, _, err := repo.GetProductBySKU(bg, "NEW"); !errors.Is(err, productsRepository.ErrVariantNotFound) {
				t.Fatalf("sku NEW leaked: %v", err)
			}
		})
	}
}

func TestUpdateProducts_SwapsSKUs(t *testing.T) {
	repo := newRepo(t)
	a := mustCreate(t, repo, product("A", nil, variant("SKU-A", 100, 1)))
	b := mustCreate(t, repo, product("B", nil, variant("SKU-B", 100, 1)))

	a.Variants[0].SKU = "SKU-B"
	b.Variants[0].SKU = "SKU-A"
	if _, err := repo.UpdateProducts(bg, []Model{a, b}); err != nil {
		t.Fatalf("UpdateProducts: %v", err)
	}

	check := func(r productsRepository.Repository) {
		t.Helper()
		gotA, _, err := r.GetProductBySKU(bg, "SKU-B")
		if err != nil || gotA.ID != a.ID {
			t.Fatalf("SKU-B -> %q, %v; want %q", gotA.ID, err, a.ID)
		}
		gotB, _, err := r.GetProductBySKU(bg, "SKU-A")
		if err != nil || gotB.ID != b.ID {
			t.Fatalf("SKU-A -> %q, %v; want %q", gotB.ID, err, b.ID)
		}
	}
	check(repo)
}

func TestUpdateProducts_SwapSurvivesReboot(t *testing.T) {
	dir := t.TempDir()
	repo, err := productsRepository.NewRepository([]string{dir, "p.db"}, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	a := mustCreate(t, repo, product("A", nil, variant("SKU-A", 100, 1)))
	b := mustCreate(t, repo, product("B", nil, variant("SKU-B", 100, 1)))
	a.Variants[0].SKU, b.Variants[0].SKU = "SKU-B", "SKU-A"
	if _, err := repo.UpdateProducts(bg, []Model{a, b}); err != nil {
		t.Fatalf("UpdateProducts: %v", err)
	}
	_ = repo.Close()

	repo, err = productsRepository.NewRepository([]string{dir, "p.db"}, 0o600)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	got, _, err := repo.GetProductBySKU(bg, "SKU-A")
	if err != nil || got.ID != b.ID {
		t.Fatalf("after reboot SKU-A -> %q, %v; want %q", got.ID, err, b.ID)
	}
}

func TestUpdateProducts_AllOrNothing(t *testing.T) {
	repo := newRepo(t)
	a := mustCreate(t, repo, product("A", nil, variant("SKU-A", 100, 1)))
	b := mustCreate(t, repo, product("B", nil, variant("SKU-B", 100, 1)))

	renamed := a
	renamed.Name = "A renamed"
	unknown := b
	unknown.ID = utils.NewULID()

	tests := []struct {
		name    string
		batch   []Model
		wantErr error
	}{
		{"unknown id", []Model{renamed, unknown}, productsRepository.ErrProductNotFound},
		{"duplicate id", []Model{renamed, renamed}, productsRepository.ErrProductDuplicateInBatch},
		{"invalid id", []Model{renamed, {ID: "bad", Name: "x", Variants: []Variant{variant("X", 1, 1)}}}, productsRepository.ErrIDInvalid},
		{"empty batch", []Model{}, productsRepository.ErrBatchEmpty},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repo.UpdateProducts(bg, tc.batch); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			got, err := repo.GetProductByID(bg, a.ID)
			if err != nil || got.Name != "A" {
				t.Fatalf("product A = %q, %v; want unchanged", got.Name, err)
			}
		})
	}
}
