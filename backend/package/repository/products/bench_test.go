package productsRepository

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

const benchProducts = 10000

var benchWords = []string{"running", "leather", "canvas", "trail", "classic", "slim", "wool", "cotton", "denim", "sport", "formal", "casual", "waterproof", "light", "premium"}
var benchCategories = []string{"shoes", "bags", "shirts", "jackets", "watches", "sports", "accessories", "jeans"}

func seedBench(b *testing.B) (string, string) {
	b.Helper()
	path := filepath.Join(b.TempDir(), "products.db")
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()

	var firstID string
	err = db.Update(func(tx *bolt.Tx) error {
		products, err := tx.CreateBucketIfNotExists(bucketProducts)
		if err != nil {
			return err
		}
		bySKU, err := tx.CreateBucketIfNotExists(bucketProductsBySKU)
		if err != nil {
			return err
		}
		now, err := utils.TimeNow()
		if err != nil {
			return err
		}
		for i := 0; i < benchProducts; i++ {
			p := Model{
				ID:          utils.NewULID(),
				Name:        fmt.Sprintf("%s %s item %d", benchWords[i%len(benchWords)], benchWords[(i/7)%len(benchWords)], i),
				Description: fmt.Sprintf("a %s product for everyday %s use", benchWords[(i/3)%len(benchWords)], benchWords[(i/11)%len(benchWords)]),
				Categories:  []string{benchCategories[i%len(benchCategories)], benchCategories[(i/5)%len(benchCategories)]},
				Metadata:    map[string]any{"brand": fmt.Sprintf("brand-%d", i%50)},
				CreatedAt:   now,
				UpdatedAt:   now,
				Variants: []Variant{
					{ID: utils.NewULID(), SKU: fmt.Sprintf("SKU-%d-A", i), ShownPrice: int64(1000 + i%5000), SellingPrice: int64(900 + i%5000), Currency: "INR", StockUnits: uint64(i % 4), Attributes: map[string]string{"size": "m", "color": "blue"}},
					{ID: utils.NewULID(), SKU: fmt.Sprintf("SKU-%d-B", i), ShownPrice: int64(1200 + i%5000), SellingPrice: int64(1100 + i%5000), Currency: "INR", StockUnits: 5, Attributes: map[string]string{"size": "l", "color": "black"}},
				},
			}
			if i == 0 {
				firstID = p.ID
			}
			raw, err := utils.FromObjectToStringJSON(p)
			if err != nil {
				return err
			}
			if err := products.Put([]byte(p.ID), []byte(raw)); err != nil {
				return err
			}
			for _, v := range p.Variants {
				if err := bySKU.Put([]byte(v.SKU), []byte(p.ID+"/"+v.ID)); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		b.Fatal(err)
	}
	return path, firstID
}

func openBench(b *testing.B) (Repository, string) {
	b.Helper()
	path, firstID := seedBench(b)
	repo, err := NewRepository([]string{path}, 0o600)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = repo.Close() })
	return repo, firstID
}

func BenchmarkNewRepository10k(b *testing.B) {
	path, _ := seedBench(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		repo, err := NewRepository([]string{path}, 0o600)
		if err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		_ = repo.Close()
		b.StartTimer()
	}
}

func BenchmarkGetProductByID(b *testing.B) {
	repo, id := openBench(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.GetProductByID(ctx, id); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkListProductsByCategory(b *testing.B) {
	repo, _ := openBench(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, _, err := repo.ListProductsByCategory(ctx, "shoes", 100, 20); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchText(b *testing.B) {
	repo, _ := openBench(b)
	ctx := context.Background()
	q := SearchQuery{Text: "leather running", Limit: 20}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.SearchProducts(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSearchFiltered(b *testing.B) {
	repo, _ := openBench(b)
	ctx := context.Background()
	lo, hi := int64(1000), int64(3000)
	q := SearchQuery{Text: "wool", Categories: []string{"shoes", "bags"}, MinPrice: &lo, MaxPrice: &hi, InStockOnly: true, Sort: SortPriceAsc, Limit: 20}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.SearchProducts(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRecommendProducts(b *testing.B) {
	repo, id := openBench(b)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := repo.RecommendProducts(ctx, id, 10); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUpdateProduct(b *testing.B) {
	repo, id := openBench(b)
	ctx := context.Background()
	p, err := repo.GetProductByID(ctx, id)
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Name = fmt.Sprintf("renamed %d", i)
		if _, err := repo.UpdateProduct(ctx, p); err != nil {
			b.Fatal(err)
		}
	}
}
