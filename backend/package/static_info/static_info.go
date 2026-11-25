package staticInfo

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

//go:embed pages.yaml
var pagesYAML string

type StaticInfo interface {
	Contact(ctx context.Context) (Contact, error)
	ListPages(ctx context.Context) ([]PageSummary, error)
	GetPage(ctx context.Context, slug string) (Page, error)
}

type staticInfo struct {
	contact Contact
	order   []string
	pages   map[string]Page
}

func NewStaticInfo(cfg Config) (StaticInfo, error) {
	data, err := validateConfig(cfg)
	if err != nil {
		return nil, err
	}
	doc, err := utils.FromStringToObjectYAML[document](pagesYAML)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrContentInvalid, err)
	}
	if len(doc.Pages) == 0 {
		return nil, ErrContentInvalid
	}
	info := &staticInfo{
		contact: Contact{
			CompanyName:      data.CompanyName,
			Email:            data.SupportEmail,
			Phone:            data.SupportPhone,
			Address:          data.Address,
			GrievanceOfficer: data.GrievanceOfficer,
		},
		order: make([]string, 0, len(doc.Pages)),
		pages: make(map[string]Page, len(doc.Pages)),
	}
	for _, raw := range doc.Pages {
		page, err := renderPage(raw, data)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrContentInvalid, err)
		}
		if _, exists := info.pages[page.Slug]; exists {
			return nil, fmt.Errorf("%w: duplicate slug %q", ErrContentInvalid, page.Slug)
		}
		info.order = append(info.order, page.Slug)
		info.pages[page.Slug] = page
	}
	return info, nil
}

func (s *staticInfo) Contact(ctx context.Context) (Contact, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Contact{}, err
	}
	return s.contact, nil
}

func (s *staticInfo) ListPages(ctx context.Context) ([]PageSummary, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	summaries := make([]PageSummary, 0, len(s.order))
	for _, slug := range s.order {
		page := s.pages[slug]
		summaries = append(summaries, PageSummary{Slug: page.Slug, Title: page.Title, Summary: page.Summary})
	}
	return summaries, nil
}

func (s *staticInfo) GetPage(ctx context.Context, slug string) (Page, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Page{}, err
	}
	slug = strings.ToLower(strings.TrimSpace(slug))
	if !slugPattern.MatchString(slug) {
		return Page{}, ErrSlugInvalid
	}
	page, ok := s.pages[slug]
	if !ok {
		return Page{}, ErrPageNotFound
	}
	return clonePage(page), nil
}
