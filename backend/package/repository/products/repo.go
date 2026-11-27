package productsRepository

import (
	"context"
	dumpRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/dump"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/blevesearch/bleve/v2"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketProducts      = []byte("products")
	bucketProductsBySKU = []byte("products_by_sku")
)

const (
	openTimeout       = time.Second
	indexBatchSize    = 1000
	maxPageLimit      = 100
	maxRecommendLimit = 10
	currencyLength    = 3
)

type Repository interface {
	CreateProduct(ctx context.Context, product Model) (Model, error)
	CreateProducts(ctx context.Context, products []Model) ([]Model, error)
	UpdateProduct(ctx context.Context, product Model) (Model, error)
	UpdateProducts(ctx context.Context, products []Model) ([]Model, error)
	GetProductByID(ctx context.Context, id string) (Model, error)
	GetProductsByIDs(ctx context.Context, ids []string) ([]Model, error)
	GetProductBySKU(ctx context.Context, sku string) (Model, Variant, error)
	GetProductByVariantID(ctx context.Context, variantID string) (Model, Variant, error)
	ListProducts(ctx context.Context, offset, limit int) ([]Model, int, error)
	ListProductsByCategory(ctx context.Context, category string, offset, limit int) ([]Model, int, error)
	ListCategories(ctx context.Context) ([]CategoryCount, error)
	SearchProducts(ctx context.Context, q SearchQuery) (SearchResult, error)
	SuggestQuery(ctx context.Context, text string) (string, error)
	RecommendProducts(ctx context.Context, productID string, limit int) ([]Model, error)
	AdjustStock(ctx context.Context, changes []StockChange) ([]Model, error)
	Dump(ctx context.Context) ([]dumpRepository.Table, error)
	Close() error
}

type repository struct {
	mu    sync.RWMutex
	db    *bolt.DB
	index bleve.Index
	cache *cache
}

func NewRepository(path []string, permission os.FileMode) (Repository, error) {
	db, err := bolt.Open(filepath.Join(path...), permission, &bolt.Options{Timeout: openTimeout})
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketProducts, bucketProductsBySKU} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}

	products, err := loadProducts(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	c := newCache(len(products))
	c.bulkLoad(products)

	index, err := buildIndex(c)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &repository{
		db:    db,
		index: index,
		cache: c,
	}, nil
}

func (r *repository) CreateProduct(ctx context.Context, product Model) (Model, error) {
	return first(r.CreateProducts(ctx, []Model{product}))
}

func (r *repository) CreateProducts(ctx context.Context, products []Model) ([]Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	items, err := prepareCreate(products)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.checkSKUs(items); err != nil {
		return nil, err
	}
	if err := r.saveAll(ctx, items); err != nil {
		return nil, err
	}
	return r.applyAll(items)
}

func (r *repository) UpdateProduct(ctx context.Context, product Model) (Model, error) {
	return first(r.UpdateProducts(ctx, []Model{product}))
}

func (r *repository) UpdateProducts(ctx context.Context, products []Model) ([]Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	items, err := normalizeAll(products)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.prepareUpdate(items); err != nil {
		return nil, err
	}
	if err := r.checkSKUs(items); err != nil {
		return nil, err
	}
	if err := r.saveAll(ctx, items); err != nil {
		return nil, err
	}
	return r.applyAll(items)
}

func (r *repository) GetProductByID(ctx context.Context, id string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(id) {
		return Model{}, ErrIDInvalid
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.cache.byID[id]
	if !ok {
		return Model{}, ErrProductNotFound
	}
	return cloneModel(e.product), nil
}

func (r *repository) GetProductsByIDs(ctx context.Context, ids []string) ([]Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hydrate(ids), nil
}

func (r *repository) GetProductBySKU(ctx context.Context, sku string) (Model, Variant, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, Variant{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ref, ok := r.cache.bySKU[strings.TrimSpace(sku)]
	if !ok {
		return Model{}, Variant{}, ErrVariantNotFound
	}
	return r.variant(ref)
}

func (r *repository) GetProductByVariantID(ctx context.Context, variantID string) (Model, Variant, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, Variant{}, err
	}
	if !utils.ValidateULID(variantID) {
		return Model{}, Variant{}, ErrIDInvalid
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ref, ok := r.cache.byVariantID[variantID]
	if !ok {
		return Model{}, Variant{}, ErrVariantNotFound
	}
	return r.variant(ref)
}

func (r *repository) ListProducts(ctx context.Context, offset, limit int) ([]Model, int, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, 0, err
	}
	limit, err := checkPage(offset, limit)
	if err != nil {
		return nil, 0, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.hydrate(page(r.cache.allIDs, offset, limit)), len(r.cache.allIDs), nil
}

func (r *repository) ListProductsByCategory(ctx context.Context, category string, offset, limit int) ([]Model, int, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, 0, err
	}
	limit, err := checkPage(offset, limit)
	if err != nil {
		return nil, 0, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.cache.byCategory[normalizeCategory(category)]
	return r.hydrate(page(ids, offset, limit)), len(ids), nil
}

func (r *repository) ListCategories(ctx context.Context) ([]CategoryCount, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cache.categoryCounts(), nil
}

func (r *repository) SearchProducts(ctx context.Context, q SearchQuery) (SearchResult, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return SearchResult{}, err
	}
	limit, err := checkPage(q.Offset, q.Limit)
	if err != nil {
		return SearchResult{}, err
	}
	q.Limit = limit
	if q.MinPrice != nil && q.MaxPrice != nil && *q.MinPrice > *q.MaxPrice {
		return SearchResult{}, ErrPriceInvalid
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	res, err := r.index.SearchInContext(ctx, buildSearchRequest(q))
	if err != nil {
		return SearchResult{}, err
	}

	ids := make([]string, len(res.Hits))
	for i, hit := range res.Hits {
		ids[i] = hit.ID
	}

	out := SearchResult{
		Products:       r.hydrate(ids),
		Total:          int(res.Total),
		CategoryFacets: []CategoryCount{},
	}
	if facet, ok := res.Facets[facetCategories]; ok && facet.Terms != nil {
		for _, t := range facet.Terms.Terms() {
			out.CategoryFacets = append(out.CategoryFacets, CategoryCount{Category: t.Term, Count: t.Count})
		}
	}
	return out, nil
}

func (r *repository) SuggestQuery(ctx context.Context, text string) (string, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return "", err
	}
	tokens := searchTokens(text)
	if len(tokens) == 0 {
		return "", nil
	}
	r.mu.RLock()
	words := r.vocabulary()
	r.mu.RUnlock()
	changed := false
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		word, ok := closestWord(token, words)
		if !ok {
			out = append(out, token)
			continue
		}
		changed = changed || word != token
		out = append(out, word)
	}
	if !changed {
		return "", nil
	}
	return strings.Join(out, " "), nil
}

func (r *repository) RecommendProducts(ctx context.Context, productID string, limit int) ([]Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	if !utils.ValidateULID(productID) {
		return nil, ErrIDInvalid
	}
	if limit <= 0 {
		return nil, ErrLimitInvalid
	}
	limit = min(limit, maxRecommendLimit)

	r.mu.RLock()
	defer r.mu.RUnlock()

	src, ok := r.cache.byID[productID]
	if !ok {
		return nil, ErrProductNotFound
	}

	seen := map[string]struct{}{productID: {}}
	picked := make([]string, 0, limit)
	for _, cat := range src.product.Categories {
		ids := r.cache.byCategory[cat]
		for _, wantInStock := range []bool{true, false} {
			for _, id := range ids {
				if len(picked) == limit {
					return r.hydrate(picked), nil
				}
				if _, dup := seen[id]; dup {
					continue
				}
				if r.cache.byID[id].inStock != wantInStock {
					continue
				}
				seen[id] = struct{}{}
				picked = append(picked, id)
			}
		}
	}
	return r.hydrate(picked), nil
}

func (r *repository) Close() error {
	indexErr := r.index.Close()
	dbErr := r.db.Close()
	if dbErr != nil {
		return dbErr
	}
	return indexErr
}

func (r *repository) AdjustStock(ctx context.Context, changes []StockChange) ([]Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	merged, err := mergeStockChanges(changes)
	if err != nil {
		return nil, err
	}
	now, err := utils.TimeNow()
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	items, err := r.prepareStock(merged, now)
	if err != nil {
		return nil, err
	}
	if err := r.saveAll(ctx, items); err != nil {
		return nil, err
	}
	return r.applyAll(items)
}
