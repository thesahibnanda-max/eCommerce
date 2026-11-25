package updatePincodes

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	resourceID  = "5c2f62fe-5afa-4119-a499-fec9d604d5bd"
	pageURL     = "https://data.gov.in/resource/all-india-pincode-directory-till-last-month"
	apiURL      = "https://api.data.gov.in/resource/" + resourceID
	pageSize    = 10000
	maxAttempts = 6
	apiKeyEnv   = "DATA_GOV_IN_API_KEY"
	enableEnv   = "UPDATE_PINCODES"
)

type record struct {
	Pincode    string `json:"pincode"`
	District   string `json:"district"`
	StateName  string `json:"statename"`
	OfficeType string `json:"officetype"`
}

type page struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Total   int      `json:"total"`
	Records []record `json:"records"`
}

type state struct {
	name string
	code string
}

type place struct {
	city  string
	state state
}

var (
	pincodePattern = regexp.MustCompile(`^[1-9][0-9]{5}$`)
	apiKeyPattern  = regexp.MustCompile(`api-key=([0-9a-f]{20,})`)
	officeWeight   = map[string]int{"HO": 3, "PO": 2, "BO": 1}
	lowerWords     = map[string]struct{}{"and": {}, "of": {}, "the": {}}
	missing        = map[string]struct{}{"": {}, "NA": {}, "N/A": {}, "NULL": {}}
	states         = map[string]state{
		"ANDAMAN AND NICOBAR ISLANDS": {"Andaman and Nicobar Islands", "IN-AN"},
		"ANDHRA PRADESH":              {"Andhra Pradesh", "IN-AP"},
		"ARUNACHAL PRADESH":           {"Arunachal Pradesh", "IN-AR"},
		"ASSAM":                       {"Assam", "IN-AS"},
		"BIHAR":                       {"Bihar", "IN-BR"},
		"CHANDIGARH":                  {"Chandigarh", "IN-CH"},
		"CHHATTISGARH":                {"Chhattisgarh", "IN-CG"},
		"THE DADRA AND NAGAR HAVELI AND DAMAN AND DIU": {"Dadra and Nagar Haveli and Daman and Diu", "IN-DH"},
		"DELHI":             {"Delhi", "IN-DL"},
		"GOA":               {"Goa", "IN-GA"},
		"GUJARAT":           {"Gujarat", "IN-GJ"},
		"HARYANA":           {"Haryana", "IN-HR"},
		"HIMACHAL PRADESH":  {"Himachal Pradesh", "IN-HP"},
		"JAMMU AND KASHMIR": {"Jammu and Kashmir", "IN-JK"},
		"JHARKHAND":         {"Jharkhand", "IN-JH"},
		"KARNATAKA":         {"Karnataka", "IN-KA"},
		"KERALA":            {"Kerala", "IN-KL"},
		"LADAKH":            {"Ladakh", "IN-LA"},
		"LAKSHADWEEP":       {"Lakshadweep", "IN-LD"},
		"MADHYA PRADESH":    {"Madhya Pradesh", "IN-MP"},
		"MAHARASHTRA":       {"Maharashtra", "IN-MH"},
		"MANIPUR":           {"Manipur", "IN-MN"},
		"MEGHALAYA":         {"Meghalaya", "IN-ML"},
		"MIZORAM":           {"Mizoram", "IN-MZ"},
		"NAGALAND":          {"Nagaland", "IN-NL"},
		"ODISHA":            {"Odisha", "IN-OD"},
		"PUDUCHERRY":        {"Puducherry", "IN-PY"},
		"PUNJAB":            {"Punjab", "IN-PB"},
		"RAJASTHAN":         {"Rajasthan", "IN-RJ"},
		"SIKKIM":            {"Sikkim", "IN-SK"},
		"TAMIL NADU":        {"Tamil Nadu", "IN-TN"},
		"TELANGANA":         {"Telangana", "IN-TS"},
		"TRIPURA":           {"Tripura", "IN-TR"},
		"UTTAR PRADESH":     {"Uttar Pradesh", "IN-UP"},
		"UTTARAKHAND":       {"Uttarakhand", "IN-UK"},
		"WEST BENGAL":       {"West Bengal", "IN-WB"},
	}
)

func Test_Update(t *testing.T) {
	if os.Getenv(enableEnv) != "1" {
		t.Skipf("set %s=1 to download the pincode directory and rewrite pincodes.csv", enableEnv)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx); err != nil {
		t.Fatalf("update_pincodes: %v", err)
	}
}

func run(ctx context.Context) error {
	out, err := outputPath()
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	key, err := apiKey(ctx, client)
	if err != nil {
		return err
	}
	records, err := download(ctx, client, key)
	if err != nil {
		return err
	}
	resolved, err := build(records)
	if err != nil {
		return err
	}
	if err := write(out, resolved); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %d pincodes to %s\n", len(resolved), out)
	return nil
}

func outputPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", errors.New("cannot locate script directory")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "backend", "package", "repository", "address", "pincodes.csv"), nil
}

func fetch(ctx context.Context, client *http.Client, target string) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		body, err := fetchOnce(ctx, client, target)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		fmt.Fprintf(os.Stderr, "retry %d/%d %s: %v\n", attempt, maxAttempts, strings.SplitN(target, "?", 2)[0], err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(3*attempt) * time.Second):
		}
	}
	return nil, lastErr
}

func fetchOnce(ctx context.Context, client *http.Client, target string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func apiKey(ctx context.Context, client *http.Client) (string, error) {
	if key := strings.TrimSpace(os.Getenv(apiKeyEnv)); key != "" {
		return key, nil
	}
	body, err := fetch(ctx, client, pageURL)
	if err != nil {
		return "", err
	}
	match := apiKeyPattern.FindSubmatch(body)
	if match == nil {
		return "", fmt.Errorf("no public sample api key on %s; set %s", pageURL, apiKeyEnv)
	}
	return string(match[1]), nil
}

func download(ctx context.Context, client *http.Client, key string) ([]record, error) {
	var records []record
	total := -1
	for offset := 0; total < 0 || offset < total; offset += pageSize {
		query := url.Values{
			"api-key": {key},
			"format":  {"json"},
			"limit":   {strconv.Itoa(pageSize)},
			"offset":  {strconv.Itoa(offset)},
		}
		body, err := fetch(ctx, client, apiURL+"?"+query.Encode())
		if err != nil {
			return nil, err
		}
		var p page
		if err := json.Unmarshal(body, &p); err != nil {
			return nil, fmt.Errorf("offset %d: %w", offset, err)
		}
		if p.Status != "ok" {
			return nil, fmt.Errorf("offset %d: api status %q: %s", offset, p.Status, p.Message)
		}
		total = p.Total
		records = append(records, p.Records...)
		fmt.Fprintf(os.Stderr, "downloaded %d/%d\n", len(records), total)
	}
	if len(records) != total {
		return nil, fmt.Errorf("expected %d records, got %d", total, len(records))
	}
	return records, nil
}

func clean(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if _, ok := missing[strings.ToUpper(value)]; ok {
		return ""
	}
	return value
}

func title(value string) string {
	words := strings.Fields(strings.ToLower(value))
	for i, word := range words {
		if _, ok := lowerWords[word]; ok && i > 0 {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func winner(votes map[place]int) place {
	candidates := make([]place, 0, len(votes))
	for p := range votes {
		candidates = append(candidates, p)
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if votes[a] != votes[b] {
			return votes[a] > votes[b]
		}
		if a.city != b.city {
			return a.city < b.city
		}
		return a.state.name < b.state.name
	})
	return candidates[0]
}

func build(records []record) (map[string]place, error) {
	votes := map[string]map[place]int{}
	seen := map[string]struct{}{}
	for _, r := range records {
		pin := clean(r.Pincode)
		if !pincodePattern.MatchString(pin) {
			continue
		}
		seen[pin] = struct{}{}
		st, ok := states[strings.ToUpper(clean(r.StateName))]
		district := clean(r.District)
		if !ok || district == "" {
			continue
		}
		if votes[pin] == nil {
			votes[pin] = map[place]int{}
		}
		weight, ok := officeWeight[strings.ToUpper(clean(r.OfficeType))]
		if !ok {
			weight = 1
		}
		votes[pin][place{city: title(district), state: st}] += weight
	}
	resolved := make(map[string]place, len(seen))
	prefixVotes := map[string]map[place]int{}
	for pin, v := range votes {
		p := winner(v)
		resolved[pin] = p
		if prefixVotes[pin[:3]] == nil {
			prefixVotes[pin[:3]] = map[place]int{}
		}
		prefixVotes[pin[:3]][p]++
	}
	var unresolved []string
	for pin := range seen {
		if _, ok := resolved[pin]; ok {
			continue
		}
		if v := prefixVotes[pin[:3]]; len(v) > 0 {
			resolved[pin] = winner(v)
			continue
		}
		unresolved = append(unresolved, pin)
	}
	if len(unresolved) > 0 {
		sort.Strings(unresolved)
		return nil, fmt.Errorf("no state for pincodes: %s", strings.Join(unresolved, ", "))
	}
	if len(resolved) == 0 {
		return nil, errors.New("no pincodes resolved")
	}
	return resolved, nil
}

func write(out string, resolved map[string]place) error {
	pins := make([]string, 0, len(resolved))
	for pin := range resolved {
		pins = append(pins, pin)
	}
	sort.Strings(pins)
	tmp := out + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	rows := make([][]string, 0, len(pins)+1)
	rows = append(rows, []string{"pincode", "city", "state", "state_code"})
	for _, pin := range pins {
		p := resolved[pin]
		rows = append(rows, []string{pin, p.city, p.state.name, p.state.code})
	}
	if err := w.WriteAll(rows); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, out)
}
