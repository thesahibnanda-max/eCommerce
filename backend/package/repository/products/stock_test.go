package productsRepository_test

import (
	"errors"
	"sync"
	"testing"

	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestAdjustStock(t *testing.T) {
	repo := newRepo(t)
	a := mustCreate(t, repo, product("Kurta", []string{"tops"}, variant("K-S", 999, 5), variant("K-M", 999, 2)))
	b := mustCreate(t, repo, product("Dupatta", []string{"accessories"}, variant("D-1", 499, 1)))
	small, medium, dupatta := a.Variants[0].ID, a.Variants[1].ID, b.Variants[0].ID

	out, err := repo.AdjustStock(bg, []productsRepository.StockChange{{VariantID: small, Delta: -2}, {VariantID: dupatta, Delta: -1}, {VariantID: small, Delta: -1}})
	if err != nil || len(out) != 2 {
		t.Fatalf("reserve: %v %d", err, len(out))
	}
	_, v, _ := repo.GetProductByVariantID(bg, small)
	_, d, _ := repo.GetProductByVariantID(bg, dupatta)
	if v.StockUnits != 2 || d.StockUnits != 0 {
		t.Fatalf("stock small=%d dupatta=%d", v.StockUnits, d.StockUnits)
	}
	if _, err := repo.AdjustStock(bg, []productsRepository.StockChange{{VariantID: medium, Delta: -1}, {VariantID: dupatta, Delta: -1}}); !errors.Is(err, productsRepository.ErrInsufficientStock) {
		t.Fatalf("insufficient: %v", err)
	}
	_, m, _ := repo.GetProductByVariantID(bg, medium)
	if m.StockUnits != 2 {
		t.Fatalf("failed batch must not change anything, medium=%d", m.StockUnits)
	}
	if _, err := repo.AdjustStock(bg, []productsRepository.StockChange{{VariantID: dupatta, Delta: 3}}); err != nil {
		t.Fatalf("restore: %v", err)
	}
	_, d, _ = repo.GetProductByVariantID(bg, dupatta)
	if d.StockUnits != 3 {
		t.Fatalf("restored dupatta=%d", d.StockUnits)
	}
	found, err := repo.SearchProducts(bg, productsRepository.SearchQuery{Text: "dupatta", InStockOnly: true, Limit: 10})
	if err != nil || found.Total != 1 {
		t.Fatalf("index not refreshed after restock: %v %d", err, found.Total)
	}
	bad := [][]productsRepository.StockChange{
		nil,
		{{VariantID: "nope", Delta: 1}},
		{{VariantID: small, Delta: 0}},
		{{VariantID: utils.NewULID(), Delta: 1}},
	}
	wants := []error{productsRepository.ErrBatchEmpty, productsRepository.ErrIDInvalid, productsRepository.ErrStockChangeInvalid, productsRepository.ErrVariantNotFound}
	for i, changes := range bad {
		if _, err := repo.AdjustStock(bg, changes); !errors.Is(err, wants[i]) {
			t.Fatalf("case %d: got %v want %v", i, err, wants[i])
		}
	}
}

func TestAdjustStock_ConcurrentNeverOversells(t *testing.T) {
	repo := newRepo(t)
	p := mustCreate(t, repo, product("Saree", nil, variant("S-1", 2999, 10)))
	id := p.Variants[0].ID
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for range 25 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.AdjustStock(bg, []productsRepository.StockChange{{VariantID: id, Delta: -1}}); err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			} else if !errors.Is(err, productsRepository.ErrInsufficientStock) {
				t.Errorf("adjust: %v", err)
			}
		}()
	}
	wg.Wait()
	_, v, _ := repo.GetProductByVariantID(bg, id)
	if wins != 10 || v.StockUnits != 0 {
		t.Fatalf("wins=%d stock=%d", wins, v.StockUnits)
	}
}
