package productsService_test

import (
	"context"
	"errors"
	"testing"

	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	productsService "github.com/thesahibnanda-max/eCommerce/backend/package/services/products"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var bg = context.Background()

type fixture struct {
	svc  productsService.Service
	repo productsRepository.Repository
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	repo, err := productsRepository.NewRepository([]string{t.TempDir(), "products.db"}, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	svc, err := productsService.NewService(repo)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return fixture{svc: svc, repo: repo}
}

func (f fixture) seed(t *testing.T, name, sku string, price int64, stock uint64, categories ...string) productsRepository.Model {
	t.Helper()
	p, err := f.repo.CreateProduct(bg, productsRepository.Model{
		Name:       name,
		Categories: categories,
		Variants:   []productsRepository.Variant{{SKU: sku, ShownPrice: price, SellingPrice: price, Currency: "INR", StockUnits: stock}},
	})
	if err != nil {
		t.Fatalf("CreateProduct(%s): %v", name, err)
	}
	return p
}

func productNames(products []productsService.Product) []string {
	out := make([]string, len(products))
	for i, p := range products {
		out[i] = p.Name
	}
	return out
}

func sameStrings(a, b []string) bool {
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

func int64Ptr(v int64) *int64 { return &v }

func TestNewService(t *testing.T) {
	f := newFixture(t)
	if _, err := productsService.NewService(nil); !errors.Is(err, productsService.ErrProductsRepoRequired) {
		t.Fatalf("err = %v, want ErrProductsRepoRequired", err)
	}
	if _, err := productsService.NewService(f.repo); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestEveryMethodIsPublic(t *testing.T) {
	f := newFixture(t)
	p := f.seed(t, "Shirt", "S-1", 100, 1, "shirts")
	calls := map[string]func() error{
		"GetProductByID": func() error {
			_, err := f.svc.GetProductByID(bg, productsService.GetProductByIDRequest{ID: p.ID})
			return err
		},
		"GetProductsByIDs": func() error {
			_, err := f.svc.GetProductsByIDs(bg, productsService.GetProductsByIDsRequest{IDs: []string{p.ID}})
			return err
		},
		"GetProductBySKU": func() error {
			_, err := f.svc.GetProductBySKU(bg, productsService.GetProductBySKURequest{SKU: "S-1"})
			return err
		},
		"GetProductByVariantID": func() error {
			_, err := f.svc.GetProductByVariantID(bg, productsService.GetProductByVariantIDRequest{VariantID: p.Variants[0].ID})
			return err
		},
		"ListProducts": func() error {
			_, err := f.svc.ListProducts(bg, productsService.ListProductsRequest{Limit: 10})
			return err
		},
		"ListProductsByCategory": func() error {
			_, err := f.svc.ListProductsByCategory(bg, productsService.ListProductsByCategoryRequest{Category: "shirts", Limit: 10})
			return err
		},
		"ListCategories": func() error {
			_, err := f.svc.ListCategories(bg, productsService.ListCategoriesRequest{})
			return err
		},
		"SearchProducts": func() error {
			_, err := f.svc.SearchProducts(bg, productsService.SearchProductsRequest{Text: "shirt", Limit: 10})
			return err
		},
		"RecommendProducts": func() error {
			_, err := f.svc.RecommendProducts(bg, productsService.RecommendProductsRequest{ProductID: p.ID, Limit: 5})
			return err
		},
	}
	for name, call := range calls {
		if err := call(); err != nil {
			t.Errorf("%s without token: err = %v", name, err)
		}
	}
}

func TestCanceledContext(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(bg)
	cancel()
	if _, err := f.svc.ListCategories(ctx, productsService.ListCategoriesRequest{}); !errors.Is(err, productsService.ErrInvalidContext) {
		t.Fatalf("err = %v, want ErrInvalidContext", err)
	}
}

func TestGetters(t *testing.T) {
	f := newFixture(t)
	a := f.seed(t, "Red Shirt", "RS-1", 100, 2, "shirts")
	b := f.seed(t, "Blue Jeans", "BJ-1", 300, 0, "jeans")

	got, err := f.svc.GetProductByID(bg, productsService.GetProductByIDRequest{ID: " " + a.ID + " "})
	if err != nil || got.Product.ID != a.ID || got.Product.Name != "Red Shirt" || got.Product.Variants[0].SKU != "RS-1" {
		t.Fatalf("GetProductByID = %+v, %v", got, err)
	}

	many, err := f.svc.GetProductsByIDs(bg, productsService.GetProductsByIDsRequest{IDs: []string{b.ID, a.ID}})
	if err != nil || !sameStrings(productNames(many.Products), []string{"Blue Jeans", "Red Shirt"}) {
		t.Fatalf("GetProductsByIDs = %v, %v", productNames(many.Products), err)
	}

	bySKU, err := f.svc.GetProductBySKU(bg, productsService.GetProductBySKURequest{SKU: "BJ-1"})
	if err != nil || bySKU.Product.ID != b.ID || bySKU.Variant.SKU != "BJ-1" {
		t.Fatalf("GetProductBySKU = %+v, %v", bySKU, err)
	}

	byVariant, err := f.svc.GetProductByVariantID(bg, productsService.GetProductByVariantIDRequest{VariantID: a.Variants[0].ID})
	if err != nil || byVariant.Product.ID != a.ID || byVariant.Variant.ID != a.Variants[0].ID {
		t.Fatalf("GetProductByVariantID = %+v, %v", byVariant, err)
	}

	tooMany := make([]string, 101)
	errCases := []struct {
		name    string
		call    func() error
		wantErr error
	}{
		{"by id unknown", func() error {
			_, err := f.svc.GetProductByID(bg, productsService.GetProductByIDRequest{ID: utils.NewULID()})
			return err
		}, productsService.ErrProductNotFound},
		{"by id invalid", func() error {
			_, err := f.svc.GetProductByID(bg, productsService.GetProductByIDRequest{ID: "bad"})
			return err
		}, productsService.ErrIDInvalid},
		{"by ids empty", func() error {
			_, err := f.svc.GetProductsByIDs(bg, productsService.GetProductsByIDsRequest{})
			return err
		}, productsService.ErrBatchEmpty},
		{"by ids too many", func() error {
			_, err := f.svc.GetProductsByIDs(bg, productsService.GetProductsByIDsRequest{IDs: tooMany})
			return err
		}, productsService.ErrTooManyIDs},
		{"by sku unknown", func() error {
			_, err := f.svc.GetProductBySKU(bg, productsService.GetProductBySKURequest{SKU: "NOPE"})
			return err
		}, productsService.ErrVariantNotFound},
		{"by variant unknown", func() error {
			_, err := f.svc.GetProductByVariantID(bg, productsService.GetProductByVariantIDRequest{VariantID: utils.NewULID()})
			return err
		}, productsService.ErrVariantNotFound},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestListing(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "P1", "P-1", 100, 1, "shirts")
	f.seed(t, "P2", "P-2", 100, 1, "shirts")
	f.seed(t, "P3", "P-3", 100, 1, "jeans")

	page, err := f.svc.ListProducts(bg, productsService.ListProductsRequest{Offset: 1, Limit: 1})
	if err != nil || page.Total != 3 || len(page.Products) != 1 {
		t.Fatalf("ListProducts = %v total %d, %v", productNames(page.Products), page.Total, err)
	}

	shirts, err := f.svc.ListProductsByCategory(bg, productsService.ListProductsByCategoryRequest{Category: "shirts", Limit: 10})
	if err != nil || shirts.Total != 2 || len(shirts.Products) != 2 {
		t.Fatalf("ListProductsByCategory = %v total %d, %v", productNames(shirts.Products), shirts.Total, err)
	}

	cats, err := f.svc.ListCategories(bg, productsService.ListCategoriesRequest{})
	if err != nil || len(cats.Categories) != 2 {
		t.Fatalf("ListCategories = %+v, %v", cats, err)
	}
	counts := map[string]int{}
	for _, c := range cats.Categories {
		counts[c.Category] = c.Count
	}
	if counts["shirts"] != 2 || counts["jeans"] != 1 {
		t.Fatalf("category counts = %v", counts)
	}

	if _, err := f.svc.ListProducts(bg, productsService.ListProductsRequest{Limit: 0}); !errors.Is(err, productsService.ErrLimitInvalid) {
		t.Fatalf("limit 0 err = %v, want ErrLimitInvalid", err)
	}
	if _, err := f.svc.ListProducts(bg, productsService.ListProductsRequest{Offset: -1, Limit: 1}); !errors.Is(err, productsService.ErrOffsetInvalid) {
		t.Fatalf("offset -1 err = %v, want ErrOffsetInvalid", err)
	}
}

func TestSearchProducts(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "Cotton Shirt", "CS-1", 500, 1, "shirts")
	f.seed(t, "Linen Shirt", "LS-1", 900, 0, "shirts")
	f.seed(t, "Denim Jeans", "DJ-1", 1500, 3, "jeans")

	tests := []struct {
		name      string
		req       productsService.SearchProductsRequest
		wantNames []string
		wantTotal int
	}{
		{"text", productsService.SearchProductsRequest{Text: "shirt", Sort: "price_asc"}, []string{"Cotton Shirt", "Linen Shirt"}, 2},
		{"category", productsService.SearchProductsRequest{Categories: []string{"jeans"}}, []string{"Denim Jeans"}, 1},
		{"price range", productsService.SearchProductsRequest{MinPrice: int64Ptr(600), MaxPrice: int64Ptr(1000)}, []string{"Linen Shirt"}, 1},
		{"in stock only", productsService.SearchProductsRequest{InStockOnly: true, Sort: "price_asc"}, []string{"Cotton Shirt", "Denim Jeans"}, 2},
		{"price desc", productsService.SearchProductsRequest{Sort: "price_desc"}, []string{"Denim Jeans", "Linen Shirt", "Cotton Shirt"}, 3},
		{"price asc", productsService.SearchProductsRequest{Sort: " PRICE_ASC "}, []string{"Cotton Shirt", "Linen Shirt", "Denim Jeans"}, 3},
		{"newest", productsService.SearchProductsRequest{Sort: "newest"}, []string{"Denim Jeans", "Linen Shirt", "Cotton Shirt"}, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.Limit = 10
			res, err := f.svc.SearchProducts(bg, tc.req)
			if err != nil {
				t.Fatalf("SearchProducts: %v", err)
			}
			if res.Total != tc.wantTotal || !sameStrings(productNames(res.Products), tc.wantNames) {
				t.Fatalf("got %v (total %d), want %v (total %d)", productNames(res.Products), res.Total, tc.wantNames, tc.wantTotal)
			}
		})
	}

	res, err := f.svc.SearchProducts(bg, productsService.SearchProductsRequest{Text: "shirt", Limit: 10})
	if err != nil || len(res.CategoryFacets) != 1 || res.CategoryFacets[0].Category != "shirts" || res.CategoryFacets[0].Count != 2 {
		t.Fatalf("facets = %+v, %v", res.CategoryFacets, err)
	}

	errCases := []struct {
		name    string
		req     productsService.SearchProductsRequest
		wantErr error
	}{
		{"invalid sort", productsService.SearchProductsRequest{Sort: "cheapest", Limit: 10}, productsService.ErrSortInvalid},
		{"min above max", productsService.SearchProductsRequest{MinPrice: int64Ptr(10), MaxPrice: int64Ptr(5), Limit: 10}, productsService.ErrPriceInvalid},
		{"zero limit", productsService.SearchProductsRequest{}, productsService.ErrLimitInvalid},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.svc.SearchProducts(bg, tc.req); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestRecommendProducts(t *testing.T) {
	f := newFixture(t)
	main := f.seed(t, "Main Shirt", "M-1", 100, 1, "shirts")
	f.seed(t, "Other Shirt", "O-1", 100, 1, "shirts")
	f.seed(t, "Jeans", "J-1", 100, 1, "jeans")

	res, err := f.svc.RecommendProducts(bg, productsService.RecommendProductsRequest{ProductID: main.ID, Limit: 5})
	if err != nil {
		t.Fatalf("RecommendProducts: %v", err)
	}
	if len(res.Products) == 0 || res.Products[0].Name != "Other Shirt" {
		t.Fatalf("recommendations = %v, want Other Shirt first", productNames(res.Products))
	}
	for _, p := range res.Products {
		if p.ID == main.ID {
			t.Fatalf("recommendations include the source product")
		}
	}

	if _, err := f.svc.RecommendProducts(bg, productsService.RecommendProductsRequest{ProductID: utils.NewULID(), Limit: 5}); !errors.Is(err, productsService.ErrProductNotFound) {
		t.Fatalf("unknown id err = %v, want ErrProductNotFound", err)
	}
	if _, err := f.svc.RecommendProducts(bg, productsService.RecommendProductsRequest{ProductID: main.ID}); !errors.Is(err, productsService.ErrLimitInvalid) {
		t.Fatalf("zero limit err = %v, want ErrLimitInvalid", err)
	}
}

func TestSearchProducts_SuggestionsWhenNothingMatches(t *testing.T) {
	f := newFixture(t)
	f.seed(t, "Cotton Shirt", "CS-1", 500, 2, "shirts")
	f.seed(t, "Denim Jeans", "DJ-1", 1500, 3, "jeans")
	f.seed(t, "Old Jacket", "OJ-1", 2500, 0, "jackets")

	res, err := f.svc.SearchProducts(bg, productsService.SearchProductsRequest{Text: "shiirtz", Limit: 10})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if res.Total != 0 && res.Suggestion != "" {
		t.Fatalf("suggestions are only for empty results")
	}
	if res.Total == 0 && (res.Suggestion != "shirt" || len(res.SuggestedProducts) == 0 || res.SuggestedProducts[0].Name != "Cotton Shirt") {
		t.Fatalf("typo suggestion: %+v", res)
	}

	res, err = f.svc.SearchProducts(bg, productsService.SearchProductsRequest{Text: "laptop charger", Limit: 10})
	if err != nil || res.Total != 0 || res.Suggestion != "" {
		t.Fatalf("unrelated query: %v %+v", err, res)
	}
	got := productNames(res.SuggestedProducts)
	if !sameStrings(got, []string{"Denim Jeans", "Cotton Shirt"}) {
		t.Fatalf("fallback must be the newest in-stock products, got %v", got)
	}

	res, err = f.svc.SearchProducts(bg, productsService.SearchProductsRequest{Text: "shirt", Limit: 10})
	if err != nil || res.Total == 0 || res.Suggestion != "" || res.SuggestedProducts == nil || len(res.SuggestedProducts) != 0 {
		t.Fatalf("normal results carry no suggestions: %v %+v", err, res)
	}
}
