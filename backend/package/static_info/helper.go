package staticInfo

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func validateConfig(cfg Config) (templateData, error) {
	name := strings.TrimSpace(cfg.CompanyName)
	if name == "" {
		return templateData{}, ErrCompanyNameRequired
	}
	email, err := utils.ValidateAndNormalizeEmail(cfg.SupportEmail)
	if err != nil {
		return templateData{}, fmt.Errorf("%w: %w", ErrSupportEmailInvalid, err)
	}
	if _, err := utils.ValidatePhoneNumber(cfg.SupportPhone); err != nil {
		return templateData{}, fmt.Errorf("%w: %w", ErrSupportPhoneInvalid, err)
	}
	address := strings.TrimSpace(cfg.Address)
	if address == "" {
		return templateData{}, ErrAddressRequired
	}
	officer := strings.TrimSpace(cfg.GrievanceOfficer)
	if officer == "" {
		return templateData{}, ErrGrievanceRequired
	}
	effective, err := time.Parse(time.DateOnly, strings.TrimSpace(cfg.EffectiveDate))
	if err != nil {
		return templateData{}, fmt.Errorf("%w: %w", ErrEffectiveDateInvalid, err)
	}
	if cfg.ReturnWindowDays == 0 {
		return templateData{}, ErrReturnWindowInvalid
	}
	refund, dispatch, delivery := strings.TrimSpace(cfg.RefundDays), strings.TrimSpace(cfg.DispatchDays), strings.TrimSpace(cfg.DeliveryDays)
	if refund == "" || dispatch == "" || delivery == "" {
		return templateData{}, ErrTermsRequired
	}
	return templateData{
		CompanyName:       name,
		SupportEmail:      email.UserName + "@" + email.Domain,
		SupportPhone:      strings.TrimSpace(cfg.SupportPhone),
		Address:           address,
		GrievanceOfficer:  officer,
		EffectiveDate:     effective.Format("2 January 2006"),
		ReturnWindowDays:  cfg.ReturnWindowDays,
		RefundDays:        refund,
		DispatchDays:      dispatch,
		DeliveryDays:      delivery,
		FreeShippingAbove: rupees(cfg.FreeShippingAbove),
		ShippingFee:       rupees(cfg.ShippingFee),
	}, nil
}

func rupees(amount uint) string {
	digits := strconv.FormatUint(uint64(amount), 10)
	if len(digits) <= 3 {
		return "₹" + digits
	}
	head, tail := digits[:len(digits)-3], digits[len(digits)-3:]
	var groups []string
	for len(head) > 2 {
		groups = append([]string{head[len(head)-2:]}, groups...)
		head = head[:len(head)-2]
	}
	groups = append([]string{head}, groups...)
	return "₹" + strings.Join(groups, ",") + "," + tail
}

func render(text string, data templateData) (string, error) {
	tmpl, err := template.New("text").Option("missingkey=error").Parse(text)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

func renderAll(texts []string, data templateData) ([]string, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(texts))
	for _, text := range texts {
		rendered, err := render(text, data)
		if err != nil {
			return nil, err
		}
		out = append(out, rendered)
	}
	return out, nil
}

func renderPage(page Page, data templateData) (Page, error) {
	if !slugPattern.MatchString(page.Slug) || strings.TrimSpace(page.Title) == "" || len(page.Sections) == 0 {
		return Page{}, fmt.Errorf("%w: page %q", ErrContentInvalid, page.Slug)
	}
	title, err := render(page.Title, data)
	if err != nil {
		return Page{}, err
	}
	summary, err := render(page.Summary, data)
	if err != nil {
		return Page{}, err
	}
	sections := make([]Section, 0, len(page.Sections))
	for _, section := range page.Sections {
		heading, err := render(section.Heading, data)
		if err != nil {
			return Page{}, err
		}
		paragraphs, err := renderAll(section.Paragraphs, data)
		if err != nil {
			return Page{}, err
		}
		bullets, err := renderAll(section.Bullets, data)
		if err != nil {
			return Page{}, err
		}
		if heading == "" || (len(paragraphs) == 0 && len(bullets) == 0) {
			return Page{}, fmt.Errorf("%w: empty section in %q", ErrContentInvalid, page.Slug)
		}
		sections = append(sections, Section{Heading: heading, Paragraphs: paragraphs, Bullets: bullets})
	}
	return Page{
		Slug:          page.Slug,
		Title:         title,
		Summary:       summary,
		EffectiveDate: data.EffectiveDate,
		Sections:      sections,
	}, nil
}

func clonePage(page Page) Page {
	sections := make([]Section, len(page.Sections))
	for i, section := range page.Sections {
		sections[i] = Section{
			Heading:    section.Heading,
			Paragraphs: append([]string(nil), section.Paragraphs...),
			Bullets:    append([]string(nil), section.Bullets...),
		}
	}
	page.Sections = sections
	return page
}
