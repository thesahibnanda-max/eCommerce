package productsRepository_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

type (
	Model   = productsRepository.Model
	Variant = productsRepository.Variant
)

var bg = context.Background()

func newRepo(t *testing.T) productsRepository.Repository {
	t.Helper()
	repo, err := productsRepository.NewRepository([]string{t.TempDir(), "products.db"}, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func variant(sku string, price int64, stock uint64) Variant {
	return Variant{SKU: sku, ShownPrice: price, SellingPrice: price, Currency: "inr", StockUnits: stock}
}

func product(name string, categories []string, variants ...Variant) Model {
	return Model{Name: name, Categories: categories, Variants: variants}
}

func mustCreate(t *testing.T, repo productsRepository.Repository, p Model) Model {
	t.Helper()
	created, err := repo.CreateProduct(bg, p)
	if err != nil {
		t.Fatalf("CreateProduct(%q): %v", p.Name, err)
	}
	return created
}

func names(products []Model) []string {
	out := make([]string, len(products))
	for i, p := range products {
		out[i] = p.Name
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCreateProduct_NormalizesAndSetsServerFields(t *testing.T) {
	repo := newRepo(t)

	created := mustCreate(t, repo, Model{
		ID:          "client",
		Name:        "  Running Shoe  ",
		Description: "  light  ",
		Categories:  []string{" Shoes ", "SPORTS", "shoes", ""},
		Variants: []Variant{{
			ID:           "client-variant",
			SKU:          " RS-42 ",
			ShownPrice:   5000,
			SellingPrice: 4000,
			Currency:     " inr ",
			StockUnits:   3,
			Attributes:   map[string]string{"size": "42"},
		}},
	})

	if !utils.ValidateULID(created.ID) {
		t.Errorf("product ID %q not a ULID", created.ID)
	}
	if !utils.ValidateULID(created.Variants[0].ID) {
		t.Errorf("variant ID %q not a ULID", created.Variants[0].ID)
	}
	if created.Name != "Running Shoe" || created.Description != "light" {
		t.Errorf("name/description not trimmed: %q / %q", created.Name, created.Description)
	}
	if !equalStrings(created.Categories, []string{"shoes", "sports"}) {
		t.Errorf("Categories = %v, want [shoes sports]", created.Categories)
	}
	v := created.Variants[0]
	if v.SKU != "RS-42" || v.Currency != "INR" {
		t.Errorf("variant not normalized: %+v", v)
	}
	if created.Metadata == nil {
		t.Error("Metadata must be non-nil")
	}
	if created.CreatedAt.IsZero() || !created.CreatedAt.Equal(created.UpdatedAt) {
		t.Errorf("timestamps not set: %v / %v", created.CreatedAt, created.UpdatedAt)
	}
}

func TestCreateProduct_Errors(t *testing.T) {
	repo := newRepo(t)
	mustCreate(t, repo, product("Taken", nil, variant("TAKEN", 100, 1)))

	canceled, cancel := context.WithCancel(bg)
	cancel()

	tests := []struct {
		name    string
		ctx     context.Context
		product Model
		wantErr error
	}{
		{"nil ctx", nil, product("A", nil, variant("A", 1, 1)), productsRepository.ErrCtxInvalid},
		{"canceled ctx", canceled, product("A", nil, variant("A", 1, 1)), context.Canceled},
		{"missing name", bg, product("  ", nil, variant("A", 1, 1)), productsRepository.ErrNameRequired},
		{"no variants", bg, product("A", nil), productsRepository.ErrVariantRequired},
		{"missing sku", bg, product("A", nil, variant(" ", 1, 1)), productsRepository.ErrSKURequired},
		{"sku repeated in product", bg, product("A", nil, variant("X", 1, 1), variant(" X ", 1, 1)), productsRepository.ErrSKUDuplicate},
		{"sku used by other product", bg, product("A", nil, variant("TAKEN", 1, 1)), productsRepository.ErrSKUAlreadyExists},
		{"negative price", bg, product("A", nil, Variant{SKU: "A", ShownPrice: -1, SellingPrice: -1, Currency: "INR"}), productsRepository.ErrPriceInvalid},
		{"selling above shown", bg, product("A", nil, Variant{SKU: "A", ShownPrice: 100, SellingPrice: 200, Currency: "INR"}), productsRepository.ErrPriceInvalid},
		{"bad currency", bg, product("A", nil, Variant{SKU: "A", ShownPrice: 1, SellingPrice: 1, Currency: "RUPEE"}), productsRepository.ErrCurrencyInvalid},
		{"empty currency", bg, product("A", nil, Variant{SKU: "A", ShownPrice: 1, SellingPrice: 1}), productsRepository.ErrCurrencyInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.CreateProduct(tt.ctx, tt.product)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got.ID != "" {
				t.Errorf("returned %+v on error, want zero value", got)
			}
		})
	}

	list, total, _ := repo.ListProducts(bg, 0, 100)
	if total != 1 || len(list) != 1 {
		t.Errorf("failed creates leaked products: total=%d", total)
	}
}

func TestUpdateProduct(t *testing.T) {
	repo := newRepo(t)
	created := mustCreate(t, repo, product("Old Name", []string{"shoes"}, variant("A", 100, 1), variant("B", 200, 1)))

	update := created
	update.Name = "New Name"
	update.Categories = []string{"bags"}
	update.Variants = []Variant{
		{ID: created.Variants[0].ID, SKU: "A2", ShownPrice: 150, SellingPrice: 120, Currency: "INR", StockUnits: 5},
		variant("C", 300, 0),
	}

	updated, err := repo.UpdateProduct(bg, update)
	if err != nil {
		t.Fatalf("UpdateProduct: %v", err)
	}
	if updated.ID != created.ID || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("ID/CreatedAt changed: %+v", updated)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("UpdatedAt %v not after %v", updated.UpdatedAt, created.UpdatedAt)
	}
	if updated.Variants[0].ID != created.Variants[0].ID {
		t.Errorf("existing variant ID not kept")
	}
	if !utils.ValidateULID(updated.Variants[1].ID) {
		t.Errorf("new variant has no ULID: %q", updated.Variants[1].ID)
	}

	for _, sku := range []string{"A", "B"} {
		if _, _, err := repo.GetProductBySKU(bg, sku); !errors.Is(err, productsRepository.ErrVariantNotFound) {
			t.Errorf("old SKU %s still resolves: %v", sku, err)
		}
	}
	if _, v, err := repo.GetProductBySKU(bg, "A2"); err != nil || v.SellingPrice != 120 {
		t.Errorf("new SKU A2 = %+v, %v", v, err)
	}
	if _, _, err := repo.GetProductByVariantID(bg, created.Variants[1].ID); !errors.Is(err, productsRepository.ErrVariantNotFound) {
		t.Errorf("removed variant still resolves: %v", err)
	}

	mustCreate(t, repo, product("Reuses freed SKU", nil, variant("A", 1, 1)))

	if list, _, _ := repo.ListProductsByCategory(bg, "shoes", 0, 10); len(list) != 0 {
		t.Errorf("product still listed in old category: %v", names(list))
	}
	if list, _, _ := repo.ListProductsByCategory(bg, "bags", 0, 10); len(list) != 1 {
		t.Errorf("product not listed in new category")
	}

	if res, _ := repo.SearchProducts(bg, productsRepository.SearchQuery{Text: "old", Limit: 10}); res.Total != 0 {
		t.Errorf("search still finds old name: %v", names(res.Products))
	}
	if res, _ := repo.SearchProducts(bg, productsRepository.SearchQuery{Text: "new", Limit: 10}); res.Total != 1 {
		t.Errorf("search does not find new name: total=%d", res.Total)
	}
}

func TestUpdateProduct_Errors(t *testing.T) {
	repo := newRepo(t)
	a := mustCreate(t, repo, product("A", nil, variant("A", 1, 1)))
	mustCreate(t, repo, product("B", nil, variant("B", 1, 1)))

	stealSKU := a
	stealSKU.Variants = []Variant{{ID: a.Variants[0].ID, SKU: "B", ShownPrice: 1, SellingPrice: 1, Currency: "INR"}}

	foreignVariant := a
	foreignVariant.Variants = []Variant{{ID: utils.NewULID(), SKU: "A", ShownPrice: 1, SellingPrice: 1, Currency: "INR"}}

	missing := a
	missing.ID = utils.NewULID()

	badID := a
	badID.ID = "nope"

	noName := a
	noName.Name = ""

	tests := []struct {
		name    string
		product Model
		wantErr error
	}{
		{"sku owned by another product", stealSKU, productsRepository.ErrSKUAlreadyExists},
		{"unknown variant id", foreignVariant, productsRepository.ErrVariantNotFound},
		{"unknown product", missing, productsRepository.ErrProductNotFound},
		{"invalid id", badID, productsRepository.ErrIDInvalid},
		{"validation still applies", noName, productsRepository.ErrNameRequired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := repo.UpdateProduct(bg, tt.product); !errors.Is(err, tt.wantErr) {
				t.Errorf("err = %v, want %v", err, tt.wantErr)
			}
		})
	}

	stored, _ := repo.GetProductByID(bg, a.ID)
	if stored.Variants[0].SKU != "A" {
		t.Errorf("failed update changed product: %+v", stored)
	}
}

func TestGetters(t *testing.T) {
	repo := newRepo(t)
	a := mustCreate(t, repo, product("A", []string{"x"}, variant("A-1", 100, 1), variant("A-2", 200, 1)))
	b := mustCreate(t, repo, product("B", []string{"x"}, variant("B-1", 100, 1)))

	if got, err := repo.GetProductByID(bg, a.ID); err != nil || got.Name != "A" {
		t.Errorf("GetProductByID = %+v, %v", got, err)
	}
	if _, err := repo.GetProductByID(bg, utils.NewULID()); !errors.Is(err, productsRepository.ErrProductNotFound) {
		t.Errorf("unknown id err = %v", err)
	}
	if _, err := repo.GetProductByID(bg, "bad"); !errors.Is(err, productsRepository.ErrIDInvalid) {
		t.Errorf("bad id err = %v", err)
	}

	got, err := repo.GetProductsByIDs(bg, []string{b.ID, "bad", utils.NewULID(), a.ID})
	if err != nil || !equalStrings(names(got), []string{"B", "A"}) {
		t.Errorf("GetProductsByIDs = %v, %v; want [B A]", names(got), err)
	}

	p, v, err := repo.GetProductBySKU(bg, " A-2 ")
	if err != nil || p.ID != a.ID || v.ID != a.Variants[1].ID {
		t.Errorf("GetProductBySKU = %s/%s, %v", p.ID, v.ID, err)
	}
	if _, _, err := repo.GetProductBySKU(bg, "missing"); !errors.Is(err, productsRepository.ErrVariantNotFound) {
		t.Errorf("unknown sku err = %v", err)
	}

	p, v, err = repo.GetProductByVariantID(bg, b.Variants[0].ID)
	if err != nil || p.ID != b.ID || v.SKU != "B-1" {
		t.Errorf("GetProductByVariantID = %s/%s, %v", p.ID, v.SKU, err)
	}
	if _, _, err := repo.GetProductByVariantID(bg, "bad"); !errors.Is(err, productsRepository.ErrIDInvalid) {
		t.Errorf("bad variant id err = %v", err)
	}
}

func TestListing(t *testing.T) {
	repo := newRepo(t)
	for i := 0; i < 5; i++ {
		cats := []string{"even"}
		if i%2 == 1 {
			cats = []string{"odd"}
		}
		mustCreate(t, repo, product(fmt.Sprintf("P%d", i), append(cats, "all"), variant(fmt.Sprintf("S%d", i), 1, 1)))
	}

	list, total, err := repo.ListProducts(bg, 0, 2)
	if err != nil || total != 5 || !equalStrings(names(list), []string{"P4", "P3"}) {
		t.Errorf("ListProducts page 1 = %v total %d err %v", names(list), total, err)
	}
	list, _, _ = repo.ListProducts(bg, 4, 2)
	if !equalStrings(names(list), []string{"P0"}) {
		t.Errorf("ListProducts last page = %v", names(list))
	}
	list, _, _ = repo.ListProducts(bg, 10, 2)
	if len(list) != 0 {
		t.Errorf("offset past end = %v", names(list))
	}
	if _, _, err := repo.ListProducts(bg, 0, 1000); err != nil {
		t.Errorf("limit above max should clamp, got %v", err)
	}

	list, total, _ = repo.ListProductsByCategory(bg, " EVEN ", 0, 10)
	if total != 3 || !equalStrings(names(list), []string{"P4", "P2", "P0"}) {
		t.Errorf("ListProductsByCategory = %v total %d", names(list), total)
	}
	list, total, _ = repo.ListProductsByCategory(bg, "none", 0, 10)
	if total != 0 || len(list) != 0 {
		t.Errorf("unknown category = %v total %d", names(list), total)
	}

	cats, _ := repo.ListCategories(bg)
	want := []productsRepository.CategoryCount{{Category: "all", Count: 5}, {Category: "even", Count: 3}, {Category: "odd", Count: 2}}
	if fmt.Sprint(cats) != fmt.Sprint(want) {
		t.Errorf("ListCategories = %v, want %v", cats, want)
	}

	for _, tc := range []struct {
		offset, limit int
		want          error
	}{{-1, 10, productsRepository.ErrOffsetInvalid}, {0, 0, productsRepository.ErrLimitInvalid}, {0, -5, productsRepository.ErrLimitInvalid}} {
		if _, _, err := repo.ListProducts(bg, tc.offset, tc.limit); !errors.Is(err, tc.want) {
			t.Errorf("ListProducts(%d,%d) err = %v, want %v", tc.offset, tc.limit, err, tc.want)
		}
	}
}

func seedSearch(t *testing.T, repo productsRepository.Repository) {
	t.Helper()
	mustCreate(t, repo, Model{Name: "Trail Running Sneaker", Description: "grippy sole for mountains", Categories: []string{"shoes", "sports"},
		Variants: []Variant{{SKU: "TRS-1", ShownPrice: 6000, SellingPrice: 5000, Currency: "INR", StockUnits: 4, Attributes: map[string]string{"color": "crimson"}}}})
	mustCreate(t, repo, Model{Name: "Leather Office Shoe", Description: "formal wear", Categories: []string{"shoes"},
		Variants: []Variant{{SKU: "LOS-1", ShownPrice: 9000, SellingPrice: 9000, Currency: "INR", StockUnits: 0}}})
	mustCreate(t, repo, Model{Name: "Canvas Backpack", Description: "carry your running gear", Categories: []string{"bags"},
		Variants: []Variant{{SKU: "CB-1", ShownPrice: 2000, SellingPrice: 1500, Currency: "INR", StockUnits: 10}, {SKU: "CB-2", ShownPrice: 3000, SellingPrice: 2500, Currency: "INR", StockUnits: 0}}})
}

func TestSearchProducts(t *testing.T) {
	repo := newRepo(t)
	seedSearch(t, repo)
	price := func(v int64) *int64 { return &v }

	tests := []struct {
		name  string
		query productsRepository.SearchQuery
		want  []string
	}{
		{"name match ranks above description match", productsRepository.SearchQuery{Text: "running"}, []string{"Trail Running Sneaker", "Canvas Backpack"}},
		{"stemming", productsRepository.SearchQuery{Text: "runs"}, []string{"Trail Running Sneaker", "Canvas Backpack"}},
		{"typo in name", productsRepository.SearchQuery{Text: "sneeker"}, []string{"Trail Running Sneaker"}},
		{"case insensitive", productsRepository.SearchQuery{Text: "LEATHER"}, []string{"Leather Office Shoe"}},
		{"attribute value", productsRepository.SearchQuery{Text: "crimson"}, []string{"Trail Running Sneaker"}},
		{"exact sku", productsRepository.SearchQuery{Text: "CB-2"}, []string{"Canvas Backpack"}},
		{"no match", productsRepository.SearchQuery{Text: "laptop"}, []string{}},
		{"category filter", productsRepository.SearchQuery{Categories: []string{"Bags"}, Sort: productsRepository.SortNewest}, []string{"Canvas Backpack"}},
		{"any of categories", productsRepository.SearchQuery{Categories: []string{"sports", "bags"}, Sort: productsRepository.SortNewest}, []string{"Canvas Backpack", "Trail Running Sneaker"}},
		{"in stock only", productsRepository.SearchQuery{InStockOnly: true, Sort: productsRepository.SortNewest}, []string{"Canvas Backpack", "Trail Running Sneaker"}},
		{"price range overlaps variant", productsRepository.SearchQuery{MinPrice: price(2000), MaxPrice: price(5000), Sort: productsRepository.SortPriceAsc}, []string{"Canvas Backpack", "Trail Running Sneaker"}},
		{"max price only", productsRepository.SearchQuery{MaxPrice: price(1500)}, []string{"Canvas Backpack"}},
		{"min price only", productsRepository.SearchQuery{MinPrice: price(9000)}, []string{"Leather Office Shoe"}},
		{"text plus filters matches category", productsRepository.SearchQuery{Text: "shoe", Categories: []string{"shoes"}, InStockOnly: true}, []string{"Trail Running Sneaker"}},
		{"sort price asc", productsRepository.SearchQuery{Sort: productsRepository.SortPriceAsc}, []string{"Canvas Backpack", "Trail Running Sneaker", "Leather Office Shoe"}},
		{"sort price desc", productsRepository.SearchQuery{Sort: productsRepository.SortPriceDesc}, []string{"Leather Office Shoe", "Trail Running Sneaker", "Canvas Backpack"}},
		{"sort newest", productsRepository.SearchQuery{Sort: productsRepository.SortNewest}, []string{"Canvas Backpack", "Leather Office Shoe", "Trail Running Sneaker"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.query.Limit = 10
			res, err := repo.SearchProducts(bg, tt.query)
			if err != nil {
				t.Fatalf("SearchProducts: %v", err)
			}
			if !equalStrings(names(res.Products), tt.want) {
				t.Errorf("got %v, want %v", names(res.Products), tt.want)
			}
			if res.Total != len(tt.want) {
				t.Errorf("Total = %d, want %d", res.Total, len(tt.want))
			}
		})
	}
}

func TestSearchProducts_PaginationFacetsAndErrors(t *testing.T) {
	repo := newRepo(t)
	seedSearch(t, repo)

	res, err := repo.SearchProducts(bg, productsRepository.SearchQuery{Sort: productsRepository.SortNewest, Offset: 1, Limit: 1})
	if err != nil || res.Total != 3 || !equalStrings(names(res.Products), []string{"Leather Office Shoe"}) {
		t.Errorf("page 2 = %v total %d err %v", names(res.Products), res.Total, err)
	}

	want := []productsRepository.CategoryCount{{Category: "shoes", Count: 2}, {Category: "bags", Count: 1}, {Category: "sports", Count: 1}}
	if fmt.Sprint(res.CategoryFacets) != fmt.Sprint(want) {
		t.Errorf("facets = %v, want %v", res.CategoryFacets, want)
	}

	lo, hi := int64(500), int64(100)
	if _, err := repo.SearchProducts(bg, productsRepository.SearchQuery{MinPrice: &lo, MaxPrice: &hi, Limit: 10}); !errors.Is(err, productsRepository.ErrPriceInvalid) {
		t.Errorf("min > max err = %v", err)
	}
	if _, err := repo.SearchProducts(bg, productsRepository.SearchQuery{}); !errors.Is(err, productsRepository.ErrLimitInvalid) {
		t.Errorf("zero limit err = %v", err)
	}
	if _, err := repo.SearchProducts(bg, productsRepository.SearchQuery{Limit: 10, Offset: -1}); !errors.Is(err, productsRepository.ErrOffsetInvalid) {
		t.Errorf("negative offset err = %v", err)
	}
}

func TestRecommendProducts(t *testing.T) {
	repo := newRepo(t)
	mustCreate(t, repo, product("shoe-old-instock", []string{"shoes"}, variant("1", 1, 1)))
	mustCreate(t, repo, product("shoe-outofstock", []string{"shoes"}, variant("2", 1, 0)))
	mustCreate(t, repo, product("sports-only", []string{"sports"}, variant("3", 1, 1)))
	mustCreate(t, repo, product("bag", []string{"bags"}, variant("4", 1, 1)))
	mustCreate(t, repo, product("shoe-and-sports", []string{"sports", "shoes"}, variant("5", 1, 1)))
	mustCreate(t, repo, product("shoe-new-instock", []string{"shoes"}, variant("6", 1, 1)))
	source := mustCreate(t, repo, product("source", []string{"shoes", "sports"}, variant("7", 1, 1)))
	lonely := mustCreate(t, repo, product("lonely", nil, variant("8", 1, 1)))

	got, err := repo.RecommendProducts(bg, source.ID, 10)
	if err != nil {
		t.Fatalf("RecommendProducts: %v", err)
	}
	want := []string{"shoe-new-instock", "shoe-and-sports", "shoe-old-instock", "shoe-outofstock", "sports-only"}
	if !equalStrings(names(got), want) {
		t.Errorf("got %v, want %v", names(got), want)
	}

	got, _ = repo.RecommendProducts(bg, source.ID, 2)
	if !equalStrings(names(got), want[:2]) {
		t.Errorf("limit 2 = %v, want %v", names(got), want[:2])
	}

	got, _ = repo.RecommendProducts(bg, lonely.ID, 5)
	if len(got) != 0 {
		t.Errorf("no categories = %v, want empty", names(got))
	}

	for _, tc := range []struct {
		id    string
		limit int
		want  error
	}{
		{source.ID, 0, productsRepository.ErrLimitInvalid},
		{source.ID, -1, productsRepository.ErrLimitInvalid},
		{"bad", 5, productsRepository.ErrIDInvalid},
		{utils.NewULID(), 5, productsRepository.ErrProductNotFound},
	} {
		if _, err := repo.RecommendProducts(bg, tc.id, tc.limit); !errors.Is(err, tc.want) {
			t.Errorf("RecommendProducts(%q,%d) err = %v, want %v", tc.id, tc.limit, err, tc.want)
		}
	}
}

func TestRecommendProducts_LimitCappedAtTen(t *testing.T) {
	repo := newRepo(t)
	var source Model
	for i := 0; i < 15; i++ {
		p := mustCreate(t, repo, product(fmt.Sprintf("P%d", i), []string{"shoes"}, variant(fmt.Sprint(i), 1, 1)))
		if i == 0 {
			source = p
		}
	}
	got, err := repo.RecommendProducts(bg, source.ID, 50)
	if err != nil || len(got) != 10 {
		t.Errorf("len = %d err %v, want 10", len(got), err)
	}
	for _, p := range got {
		if p.ID == source.ID {
			t.Error("source product recommended to itself")
		}
	}
}

func TestReturnedValuesDoNotAliasCache(t *testing.T) {
	repo := newRepo(t)
	input := Model{
		Name:       "A",
		Categories: []string{"x"},
		Metadata:   map[string]any{"tags": []any{"a"}, "nested": map[string]any{"k": "v"}},
		Variants:   []Variant{{SKU: "A", ShownPrice: 1, SellingPrice: 1, Currency: "INR", Attributes: map[string]string{"size": "M"}, ImageURLs: []string{"u1"}}},
	}
	created := mustCreate(t, repo, input)

	input.Metadata["tags"].([]any)[0] = "mutated"
	input.Variants[0].Attributes["size"] = "mutated"
	created.Name = "mutated"
	created.Categories[0] = "mutated"
	created.Metadata["nested"].(map[string]any)["k"] = "mutated"
	created.Variants[0].ImageURLs[0] = "mutated"

	got, _ := repo.GetProductByID(bg, created.ID)
	got.Variants[0].SKU = "mutated"

	stored, _ := repo.GetProductByID(bg, created.ID)
	if stored.Name != "A" || stored.Categories[0] != "x" || stored.Variants[0].SKU != "A" ||
		stored.Variants[0].Attributes["size"] != "M" || stored.Variants[0].ImageURLs[0] != "u1" ||
		stored.Metadata["tags"].([]any)[0] != "a" || stored.Metadata["nested"].(map[string]any)["k"] != "v" {
		t.Errorf("cache was mutated through a returned or input value: %+v", stored)
	}
}

func TestBootRebuildsCacheAndSearch(t *testing.T) {
	path := []string{t.TempDir(), "products.db"}
	repo, err := productsRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	a := mustCreate(t, repo, Model{Name: "Trail Sneaker", Categories: []string{"shoes"}, Metadata: map[string]any{"brand": "x"},
		Variants: []Variant{{SKU: "TS", ShownPrice: 10, SellingPrice: 10, Currency: "INR", StockUnits: 1}}})
	b := mustCreate(t, repo, product("Office Shoe", []string{"shoes"}, variant("OS", 20, 0)))
	upd := b
	upd.Name = "Formal Shoe"
	if _, err := repo.UpdateProduct(bg, upd); err != nil {
		t.Fatalf("UpdateProduct: %v", err)
	}
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	reopened, err := productsRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	got, err := reopened.GetProductByID(bg, a.ID)
	if err != nil || got.Name != "Trail Sneaker" || got.Metadata["brand"] != "x" || !got.CreatedAt.Equal(a.CreatedAt) {
		t.Errorf("GetProductByID after reopen = %+v, %v", got, err)
	}
	if _, v, err := reopened.GetProductBySKU(bg, "OS"); err != nil || v.ID != b.Variants[0].ID {
		t.Errorf("SKU index after reopen: %v", err)
	}
	if list, total, _ := reopened.ListProductsByCategory(bg, "shoes", 0, 10); total != 2 || !equalStrings(names(list), []string{"Formal Shoe", "Trail Sneaker"}) {
		t.Errorf("category list after reopen = %v", names(list))
	}
	if res, _ := reopened.SearchProducts(bg, productsRepository.SearchQuery{Text: "formal", Limit: 10}); res.Total != 1 {
		t.Errorf("search after reopen total = %d, want 1", res.Total)
	}
	if res, _ := reopened.SearchProducts(bg, productsRepository.SearchQuery{Text: "office", Limit: 10}); res.Total != 0 {
		t.Errorf("stale name found after reopen")
	}
	if _, err := reopened.CreateProduct(bg, product("dup", nil, variant("TS", 1, 1))); !errors.Is(err, productsRepository.ErrSKUAlreadyExists) {
		t.Errorf("SKU uniqueness after reopen err = %v", err)
	}
	if recs, _ := reopened.RecommendProducts(bg, a.ID, 5); !equalStrings(names(recs), []string{"Formal Shoe"}) {
		t.Errorf("recommendations after reopen = %v", names(recs))
	}
}

func TestNewRepository_LockedFileTimesOut(t *testing.T) {
	path := []string{t.TempDir(), "products.db"}
	first, err := productsRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	defer first.Close()
	if _, err := productsRepository.NewRepository(path, 0o600); err == nil {
		t.Error("second open of a locked file succeeded, want timeout error")
	}
}

func TestConcurrentReadsAndWrites(t *testing.T) {
	repo := newRepo(t)
	seed := mustCreate(t, repo, product("seed", []string{"shoes"}, variant("SEED", 1, 1)))

	var wg sync.WaitGroup
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				if _, err := repo.CreateProduct(bg, product(fmt.Sprintf("shoe %d-%d", w, i), []string{"shoes"}, variant(fmt.Sprintf("W%d-%d", w, i), int64(i+1), 1))); err != nil {
					t.Errorf("create: %v", err)
				}
			}
		}(w)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			upd := seed
			upd.Name = fmt.Sprintf("seed v%d", i)
			if _, err := repo.UpdateProduct(bg, upd); err != nil {
				t.Errorf("update: %v", err)
			}
		}
	}()
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				if _, err := repo.SearchProducts(bg, productsRepository.SearchQuery{Text: "shoe", Limit: 10}); err != nil {
					t.Errorf("search: %v", err)
				}
				if _, err := repo.RecommendProducts(bg, seed.ID, 10); err != nil {
					t.Errorf("recommend: %v", err)
				}
				if _, _, err := repo.ListProductsByCategory(bg, "shoes", 0, 10); err != nil {
					t.Errorf("list: %v", err)
				}
			}
		}()
	}
	wg.Wait()

	_, total, _ := repo.ListProductsByCategory(bg, "shoes", 0, 1)
	if total != 41 {
		t.Errorf("total = %d, want 41", total)
	}
	res, _ := repo.SearchProducts(bg, productsRepository.SearchQuery{Categories: []string{"shoes"}, Limit: 1})
	if res.Total != 41 {
		t.Errorf("search total = %d, want 41 (index out of sync)", res.Total)
	}
}
