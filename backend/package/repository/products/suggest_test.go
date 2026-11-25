package productsRepository_test

import (
	"testing"

	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
)

func TestSearch_PrefixTyposAndSuggestions(t *testing.T) {
	repo := newRepo(t)
	mustCreate(t, repo, product("Rose Chikankari Kurta", []string{"kurtas"}, variant("RK-1", 1199, 4)))
	mustCreate(t, repo, product("Banarasi Silk Saree", []string{"sarees"}, variant("BS-1", 7499, 2)))
	mustCreate(t, repo, product("Oxidised Jhumkas", []string{"accessories"}, variant("OJ-1", 449, 9)))

	cases := map[string]string{
		"kur":      "Rose Chikankari Kurta",
		"chikan":   "Rose Chikankari Kurta",
		"banarsi":  "Banarasi Silk Saree",
		"accessor": "Oxidised Jhumkas",
	}
	for text, want := range cases {
		res, err := repo.SearchProducts(bg, productsRepository.SearchQuery{Text: text, Limit: 10})
		if err != nil || res.Total == 0 || res.Products[0].Name != want {
			t.Fatalf("%q: %v total=%d first=%v", text, err, res.Total, names(res.Products))
		}
	}

	none, err := repo.SearchProducts(bg, productsRepository.SearchQuery{Text: "zzzzqqq", Limit: 10})
	if err != nil || none.Total != 0 {
		t.Fatalf("nonsense query must find nothing: %v %d", err, none.Total)
	}

	suggestions := map[string]string{
		"jhumkaz kurtta": "jhumkas kurta",
		"sareee":         "saree",
		"rose":           "",
		"zzzzqqq":        "",
		"   ":            "",
	}
	for text, want := range suggestions {
		got, err := repo.SuggestQuery(bg, text)
		if err != nil || got != want {
			t.Fatalf("SuggestQuery(%q) = %q, %v; want %q", text, got, err, want)
		}
	}

	relaxed, err := repo.SearchProducts(bg, productsRepository.SearchQuery{Text: "sillk sareeez", Relaxed: true, Limit: 10})
	if err != nil || relaxed.Total == 0 {
		t.Fatalf("relaxed search should still find the saree: %v", err)
	}
}
