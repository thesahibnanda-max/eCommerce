package productsRepository

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/lang/en"
	"github.com/blevesearch/bleve/v2/index/scorch"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

func loadProducts(db *bolt.DB) ([]Model, error) {
	var products []Model
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketProducts)
		products = make([]Model, 0, b.Stats().KeyN)
		return b.ForEach(func(k, v []byte) error {
			p, err := utils.FromStringToObjectJSON[Model](string(v))
			if err != nil {
				return fmt.Errorf("productsRepository: decode product %s: %w", k, err)
			}
			products = append(products, p)
			return nil
		})
	})
	return products, err
}

func buildIndex(c *cache) (bleve.Index, error) {
	index, err := newSearchIndex()
	if err != nil {
		return nil, err
	}
	batch := index.NewBatch()
	for _, id := range c.allIDs {
		if err := batch.Index(id, toSearchDoc(c.byID[id])); err != nil {
			_ = index.Close()
			return nil, err
		}
		if batch.Size() >= indexBatchSize {
			if err := index.Batch(batch); err != nil {
				_ = index.Close()
				return nil, err
			}
			batch.Reset()
		}
	}
	if batch.Size() > 0 {
		if err := index.Batch(batch); err != nil {
			_ = index.Close()
			return nil, err
		}
	}
	return index, nil
}

type pending struct {
	product Model
	old     *Model
}

func first(products []Model, err error) (Model, error) {
	if len(products) == 0 {
		return Model{}, err
	}
	return products[0], err
}

func normalizeAll(products []Model) ([]pending, error) {
	if len(products) == 0 {
		return nil, ErrBatchEmpty
	}
	items := make([]pending, len(products))
	for i, in := range products {
		p, err := normalizeProduct(in)
		if err != nil {
			return nil, err
		}
		items[i] = pending{product: p}
	}
	return items, nil
}

func prepareCreate(products []Model) ([]pending, error) {
	items, err := normalizeAll(products)
	if err != nil {
		return nil, err
	}
	now, err := utils.TimeNow()
	if err != nil {
		return nil, err
	}
	for i := range items {
		p := &items[i].product
		p.ID = utils.NewULID()
		p.CreatedAt = now
		p.UpdatedAt = now
		for j := range p.Variants {
			p.Variants[j].ID = utils.NewULID()
		}
	}
	return items, nil
}

func (r *repository) prepareUpdate(items []pending) error {
	now, err := utils.TimeNow()
	if err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(items))
	for i := range items {
		p := &items[i].product
		if !utils.ValidateULID(p.ID) {
			return ErrIDInvalid
		}
		if _, dup := seen[p.ID]; dup {
			return ErrProductDuplicateInBatch
		}
		seen[p.ID] = struct{}{}

		old, ok := r.cache.byID[p.ID]
		if !ok {
			return ErrProductNotFound
		}
		existing := make(map[string]struct{}, len(old.product.Variants))
		for _, v := range old.product.Variants {
			existing[v.ID] = struct{}{}
		}
		for j, v := range p.Variants {
			if v.ID == "" {
				p.Variants[j].ID = utils.NewULID()
				continue
			}
			if _, ok := existing[v.ID]; !ok {
				return ErrVariantNotFound
			}
		}
		p.CreatedAt = old.product.CreatedAt
		p.UpdatedAt = now
		oldProduct := old.product
		items[i].old = &oldProduct
	}
	return nil
}

func mergeStockChanges(changes []StockChange) ([]StockChange, error) {
	if len(changes) == 0 {
		return nil, ErrBatchEmpty
	}
	merged := make([]StockChange, 0, len(changes))
	index := make(map[string]int, len(changes))
	for _, change := range changes {
		variantID := strings.TrimSpace(change.VariantID)
		if !utils.ValidateULID(variantID) {
			return nil, ErrIDInvalid
		}
		if change.Delta == 0 {
			return nil, ErrStockChangeInvalid
		}
		if i, ok := index[variantID]; ok {
			merged[i].Delta += change.Delta
			continue
		}
		index[variantID] = len(merged)
		merged = append(merged, StockChange{VariantID: variantID, Delta: change.Delta})
	}
	return merged, nil
}

func (r *repository) prepareStock(changes []StockChange, now time.Time) ([]pending, error) {
	byProduct := map[string]int{}
	var items []pending
	for _, change := range changes {
		ref, ok := r.cache.byVariantID[change.VariantID]
		if !ok {
			return nil, ErrVariantNotFound
		}
		i, seen := byProduct[ref.productID]
		if !seen {
			old := cloneModel(r.cache.byID[ref.productID].product)
			updated := cloneModel(old)
			updated.UpdatedAt = now
			i = len(items)
			byProduct[ref.productID] = i
			items = append(items, pending{product: updated, old: &old})
		}
		product := &items[i].product
		for j := range product.Variants {
			if product.Variants[j].ID != change.VariantID {
				continue
			}
			current := product.Variants[j].StockUnits
			if change.Delta < 0 && uint64(-change.Delta) > current {
				return nil, ErrInsufficientStock
			}
			if change.Delta > 0 {
				product.Variants[j].StockUnits = current + uint64(change.Delta)
			} else {
				product.Variants[j].StockUnits = current - uint64(-change.Delta)
			}
		}
	}
	return items, nil
}

func (r *repository) checkSKUs(items []pending) error {
	inBatch := make(map[string]struct{}, len(items))
	for _, it := range items {
		inBatch[it.product.ID] = struct{}{}
	}
	skus := make(map[string]struct{})
	for _, it := range items {
		for _, v := range it.product.Variants {
			if _, dup := skus[v.SKU]; dup {
				return ErrSKUDuplicateInBatch
			}
			skus[v.SKU] = struct{}{}
			if ref, taken := r.cache.bySKU[v.SKU]; taken {
				if _, ok := inBatch[ref.productID]; !ok {
					return ErrSKUAlreadyExists
				}
			}
		}
	}
	return nil
}

func (r *repository) saveAll(ctx context.Context, items []pending) error {
	raws := make([]string, len(items))
	for i, it := range items {
		raw, err := utils.FromObjectToStringJSON(it.product)
		if err != nil {
			return fmt.Errorf("productsRepository: encode product %s: %w", it.product.ID, err)
		}
		raws[i] = raw
	}
	return r.db.Update(func(tx *bolt.Tx) error {
		if err := utils.CheckContext(ctx); err != nil {
			return err
		}
		bySKU := tx.Bucket(bucketProductsBySKU)
		for _, it := range items {
			if it.old == nil {
				continue
			}
			for _, v := range it.old.Variants {
				if err := bySKU.Delete([]byte(v.SKU)); err != nil {
					return err
				}
			}
		}
		products := tx.Bucket(bucketProducts)
		for i, it := range items {
			for _, v := range it.product.Variants {
				if bySKU.Get([]byte(v.SKU)) != nil {
					return ErrSKUAlreadyExists
				}
				if err := bySKU.Put([]byte(v.SKU), []byte(it.product.ID+"/"+v.ID)); err != nil {
					return err
				}
			}
			if err := products.Put([]byte(it.product.ID), []byte(raws[i])); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *repository) applyAll(items []pending) ([]Model, error) {
	out := make([]Model, len(items))
	batch := r.index.NewBatch()
	for i, it := range items {
		r.cache.put(it.product)
		if err := batch.Index(it.product.ID, toSearchDoc(r.cache.byID[it.product.ID])); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrSearchIndex, err)
		}
		out[i] = cloneModel(it.product)
	}
	if err := r.index.Batch(batch); err != nil {
		return out, fmt.Errorf("%w: %w", ErrSearchIndex, err)
	}
	return out, nil
}

func (r *repository) variant(ref variantRef) (Model, Variant, error) {
	e, v, ok := r.cache.variant(ref)
	if !ok {
		return Model{}, Variant{}, ErrVariantNotFound
	}
	return cloneModel(e.product), cloneVariant(v), nil
}

func (r *repository) hydrate(ids []string) []Model {
	out := make([]Model, 0, len(ids))
	for _, id := range ids {
		if e, ok := r.cache.byID[id]; ok {
			out = append(out, cloneModel(e.product))
		}
	}
	return out
}

func normalizeProduct(in Model) (Model, error) {
	p := cloneModel(in)
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return Model{}, ErrNameRequired
	}
	p.Description = strings.TrimSpace(p.Description)
	if p.Metadata == nil {
		p.Metadata = make(map[string]any)
	}
	p.Categories = normalizeCategories(p.Categories)

	if len(p.Variants) == 0 {
		return Model{}, ErrVariantRequired
	}
	skus := make(map[string]struct{}, len(p.Variants))
	for i := range p.Variants {
		v := &p.Variants[i]
		v.ID = strings.TrimSpace(v.ID)
		v.SKU = strings.TrimSpace(v.SKU)
		if v.SKU == "" {
			return Model{}, ErrSKURequired
		}
		if _, dup := skus[v.SKU]; dup {
			return Model{}, ErrSKUDuplicate
		}
		skus[v.SKU] = struct{}{}
		if v.ShownPrice < 0 || v.SellingPrice < 0 || v.SellingPrice > v.ShownPrice {
			return Model{}, ErrPriceInvalid
		}
		v.Currency = strings.ToUpper(strings.TrimSpace(v.Currency))
		if !isCurrency(v.Currency) {
			return Model{}, ErrCurrencyInvalid
		}
	}
	return p, nil
}

func normalizeCategory(category string) string {
	return strings.ToLower(strings.TrimSpace(category))
}

func normalizeCategories(categories []string) []string {
	out := make([]string, 0, len(categories))
	seen := make(map[string]struct{}, len(categories))
	for _, c := range categories {
		c = normalizeCategory(c)
		if c == "" {
			continue
		}
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

func isCurrency(s string) bool {
	if len(s) != currencyLength {
		return false
	}
	for _, ch := range s {
		if ch < 'A' || ch > 'Z' {
			return false
		}
	}
	return true
}

func checkPage(offset, limit int) (int, error) {
	if offset < 0 {
		return 0, ErrOffsetInvalid
	}
	if limit <= 0 {
		return 0, ErrLimitInvalid
	}
	return min(limit, maxPageLimit), nil
}

func newCache(capacity int) *cache {
	return &cache{
		byID:        make(map[string]*entry, capacity),
		bySKU:       make(map[string]variantRef, capacity),
		byVariantID: make(map[string]variantRef, capacity),
		allIDs:      make([]string, 0, capacity),
		byCategory:  make(map[string][]string),
	}
}

func newEntry(p Model) *entry {
	e := &entry{product: p}
	for i, v := range p.Variants {
		if i == 0 || v.SellingPrice < e.minPrice {
			e.minPrice = v.SellingPrice
		}
		if i == 0 || v.SellingPrice > e.maxPrice {
			e.maxPrice = v.SellingPrice
		}
		if v.StockUnits > 0 {
			e.inStock = true
		}
	}
	return e
}

func (c *cache) bulkLoad(products []Model) {
	for _, p := range products {
		c.byID[p.ID] = newEntry(p)
		c.allIDs = append(c.allIDs, p.ID)
		c.addRefs(p)
		for _, cat := range p.Categories {
			c.byCategory[cat] = append(c.byCategory[cat], p.ID)
		}
	}
	sortDesc(c.allIDs)
	for _, ids := range c.byCategory {
		sortDesc(ids)
	}
}

func (c *cache) put(p Model) {
	old, exists := c.byID[p.ID]
	if exists {
		c.removeRefs(old.product)
		for _, cat := range old.product.Categories {
			c.byCategory[cat] = removeDesc(c.byCategory[cat], p.ID)
			if len(c.byCategory[cat]) == 0 {
				delete(c.byCategory, cat)
			}
		}
	} else {
		c.allIDs = insertDesc(c.allIDs, p.ID)
	}
	c.byID[p.ID] = newEntry(p)
	c.addRefs(p)
	for _, cat := range p.Categories {
		c.byCategory[cat] = insertDesc(c.byCategory[cat], p.ID)
	}
}

func (c *cache) addRefs(p Model) {
	for _, v := range p.Variants {
		ref := variantRef{productID: p.ID, variantID: v.ID}
		c.bySKU[v.SKU] = ref
		c.byVariantID[v.ID] = ref
	}
}

func (c *cache) removeRefs(p Model) {
	for _, v := range p.Variants {
		if ref, ok := c.bySKU[v.SKU]; ok && ref.productID == p.ID {
			delete(c.bySKU, v.SKU)
		}
		if ref, ok := c.byVariantID[v.ID]; ok && ref.productID == p.ID {
			delete(c.byVariantID, v.ID)
		}
	}
}

func (c *cache) variant(ref variantRef) (*entry, Variant, bool) {
	e, ok := c.byID[ref.productID]
	if !ok {
		return nil, Variant{}, false
	}
	for _, v := range e.product.Variants {
		if v.ID == ref.variantID {
			return e, v, true
		}
	}
	return nil, Variant{}, false
}

func (c *cache) categoryCounts() []CategoryCount {
	counts := make([]CategoryCount, 0, len(c.byCategory))
	for cat, ids := range c.byCategory {
		counts = append(counts, CategoryCount{Category: cat, Count: len(ids)})
	}
	sort.Slice(counts, func(i, j int) bool {
		if counts[i].Count != counts[j].Count {
			return counts[i].Count > counts[j].Count
		}
		return counts[i].Category < counts[j].Category
	})
	return counts
}

func sortDesc(ids []string) {
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
}

func searchDesc(ids []string, id string) int {
	return sort.Search(len(ids), func(i int) bool { return strings.Compare(ids[i], id) <= 0 })
}

func insertDesc(ids []string, id string) []string {
	i := searchDesc(ids, id)
	if i < len(ids) && ids[i] == id {
		return ids
	}
	ids = append(ids, "")
	copy(ids[i+1:], ids[i:])
	ids[i] = id
	return ids
}

func removeDesc(ids []string, id string) []string {
	i := searchDesc(ids, id)
	if i < len(ids) && ids[i] == id {
		return append(ids[:i], ids[i+1:]...)
	}
	return ids
}

func page(ids []string, offset, limit int) []string {
	if offset >= len(ids) {
		return nil
	}
	return ids[offset:min(offset+limit, len(ids))]
}

func cloneModel(p Model) Model {
	out := p
	out.Metadata = cloneMap(p.Metadata)
	if p.Categories != nil {
		out.Categories = append([]string(nil), p.Categories...)
	}
	if p.Variants != nil {
		out.Variants = make([]Variant, len(p.Variants))
		for i, v := range p.Variants {
			out.Variants[i] = cloneVariant(v)
		}
	}
	return out
}

func cloneVariant(v Variant) Variant {
	out := v
	if v.Attributes != nil {
		out.Attributes = make(map[string]string, len(v.Attributes))
		maps.Copy(out.Attributes, v.Attributes)
	}
	if v.ImageURLs != nil {
		out.ImageURLs = append([]string(nil), v.ImageURLs...)
	}
	return out
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return cloneMap(t)
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = cloneAny(item)
		}
		return out
	default:
		return v
	}
}

func newSearchIndex() (bleve.Index, error) {
	return bleve.NewUsing("", newIndexMapping(), scorch.Name, scorch.Name, nil)
}

func newIndexMapping() mapping.IndexMapping {
	text := bleve.NewTextFieldMapping()
	text.Analyzer = en.AnalyzerName
	text.Store = false
	text.IncludeInAll = false
	text.IncludeTermVectors = false
	text.DocValues = false

	keyword := bleve.NewKeywordFieldMapping()
	keyword.Store = false
	keyword.IncludeInAll = false
	keyword.IncludeTermVectors = false

	number := bleve.NewNumericFieldMapping()
	number.Store = false
	number.IncludeInAll = false

	boolean := bleve.NewBooleanFieldMapping()
	boolean.Store = false
	boolean.IncludeInAll = false

	doc := bleve.NewDocumentStaticMapping()
	doc.AddFieldMappingsAt(fieldName, text)
	doc.AddFieldMappingsAt(fieldDescription, text)
	doc.AddFieldMappingsAt(fieldAttributes, text)
	doc.AddFieldMappingsAt(fieldCategories, keyword)
	doc.AddFieldMappingsAt(fieldSKUs, keyword)
	doc.AddFieldMappingsAt(fieldMinPrice, number)
	doc.AddFieldMappingsAt(fieldMaxPrice, number)
	doc.AddFieldMappingsAt(fieldInStock, boolean)

	m := bleve.NewIndexMapping()
	m.DefaultMapping = doc
	m.DefaultAnalyzer = en.AnalyzerName
	m.StoreDynamic = false
	m.IndexDynamic = false
	m.DocValuesDynamic = false
	return m
}

func toSearchDoc(e *entry) searchDoc {
	p := e.product
	attributes := make([]string, 0, len(p.Variants))
	skus := make([]string, 0, len(p.Variants))
	for _, v := range p.Variants {
		skus = append(skus, v.SKU)
		keys := make([]string, 0, len(v.Attributes))
		for k := range v.Attributes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			attributes = append(attributes, v.Attributes[k])
		}
	}
	return searchDoc{
		Name:        p.Name,
		Description: p.Description,
		Attributes:  strings.Join(attributes, " "),
		Categories:  p.Categories,
		SKUs:        skus,
		MinPrice:    float64(e.minPrice),
		MaxPrice:    float64(e.maxPrice),
		InStock:     e.inStock,
	}
}

func searchTokens(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	tokens := make([]string, 0, len(fields))
	for _, field := range fields {
		if utf8.RuneCountInString(field) >= minTokenLength {
			tokens = append(tokens, field)
		}
	}
	return tokens
}

func textSearchQuery(text string, relaxed bool) query.Query {
	tokens := searchTokens(text)
	fuzziness := nameFuzziness
	for _, token := range tokens {
		if relaxed || utf8.RuneCountInString(token) >= wideTokenLength {
			fuzziness = wideFuzziness
		}
	}

	name := bleve.NewMatchQuery(text)
	name.SetField(fieldName)
	name.SetBoost(nameBoost)
	name.SetFuzziness(fuzziness)

	description := bleve.NewMatchQuery(text)
	description.SetField(fieldDescription)
	if relaxed {
		description.SetFuzziness(wideFuzziness)
	}

	attributes := bleve.NewMatchQuery(text)
	attributes.SetField(fieldAttributes)

	sku := bleve.NewTermQuery(strings.TrimSpace(text))
	sku.SetField(fieldSKUs)
	sku.SetBoost(nameBoost)

	disjuncts := []query.Query{name, description, attributes, sku}
	for _, token := range tokens {
		prefix := bleve.NewPrefixQuery(token)
		prefix.SetField(fieldName)
		prefix.SetBoost(prefixBoost)
		category := bleve.NewPrefixQuery(token)
		category.SetField(fieldCategories)
		disjuncts = append(disjuncts, prefix, category)
		if relaxed {
			descriptionPrefix := bleve.NewPrefixQuery(token)
			descriptionPrefix.SetField(fieldDescription)
			disjuncts = append(disjuncts, descriptionPrefix)
		}
	}
	return bleve.NewDisjunctionQuery(disjuncts...)
}

func buildSearchRequest(q SearchQuery) *bleve.SearchRequest {
	var textQuery query.Query = bleve.NewMatchAllQuery()
	if text := strings.TrimSpace(q.Text); text != "" {
		textQuery = textSearchQuery(text, q.Relaxed)
	}

	conjuncts := []query.Query{textQuery}

	if categories := normalizeCategories(q.Categories); len(categories) > 0 {
		terms := make([]query.Query, 0, len(categories))
		for _, cat := range categories {
			term := bleve.NewTermQuery(cat)
			term.SetField(fieldCategories)
			terms = append(terms, term)
		}
		conjuncts = append(conjuncts, bleve.NewDisjunctionQuery(terms...))
	}

	inclusive := true
	if q.MaxPrice != nil {
		hi := float64(*q.MaxPrice)
		r := bleve.NewNumericRangeInclusiveQuery(nil, &hi, nil, &inclusive)
		r.SetField(fieldMinPrice)
		conjuncts = append(conjuncts, r)
	}
	if q.MinPrice != nil {
		lo := float64(*q.MinPrice)
		r := bleve.NewNumericRangeInclusiveQuery(&lo, nil, &inclusive, nil)
		r.SetField(fieldMaxPrice)
		conjuncts = append(conjuncts, r)
	}

	if q.InStockOnly {
		b := bleve.NewBoolFieldQuery(true)
		b.SetField(fieldInStock)
		conjuncts = append(conjuncts, b)
	}

	req := bleve.NewSearchRequestOptions(bleve.NewConjunctionQuery(conjuncts...), q.Limit, q.Offset, false)
	req.AddFacet(facetCategories, bleve.NewFacetRequest(fieldCategories, facetSize))

	switch q.Sort {
	case SortPriceAsc:
		req.SortBy([]string{fieldMinPrice, "-_id"})
	case SortPriceDesc:
		req.SortBy([]string{"-" + fieldMinPrice, "-_id"})
	case SortNewest:
		req.SortBy([]string{"-_id"})
	default:
		req.SortBy([]string{"-_score", "-_id"})
	}
	return req
}

func (r *repository) vocabulary() map[string]struct{} {
	words := map[string]struct{}{}
	for _, id := range r.cache.allIDs {
		e := r.cache.byID[id]
		if e == nil {
			continue
		}
		for _, token := range searchTokens(e.product.Name) {
			if utf8.RuneCountInString(token) >= minSuggestLength {
				words[token] = struct{}{}
			}
		}
		for _, category := range e.product.Categories {
			for _, token := range searchTokens(category) {
				if utf8.RuneCountInString(token) >= minSuggestLength {
					words[token] = struct{}{}
				}
			}
		}
	}
	return words
}

func closestWord(token string, words map[string]struct{}) (string, bool) {
	if _, exact := words[token]; exact {
		return token, true
	}
	limit := max(1, utf8.RuneCountInString(token)/3)
	best, bestDistance := "", limit+1
	for word := range words {
		distance := editDistance(token, word)
		if strings.HasPrefix(word, token) {
			distance = min(distance, 1)
		}
		if distance < bestDistance || (distance == bestDistance && word < best) {
			best, bestDistance = word, distance
		}
	}
	return best, bestDistance <= limit
}

func editDistance(a, b string) int {
	left, right := []rune(a), []rune(b)
	previous := make([]int, len(right)+1)
	current := make([]int, len(right)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(left); i++ {
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 1
			if left[i-1] == right[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous, current = current, previous
	}
	return previous[len(right)]
}
