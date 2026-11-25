package staticInfo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	staticInfo "github.com/thesahibnanda-max/eCommerce/backend/package/static_info"
)

func validConfig() staticInfo.Config {
	return staticInfo.Config{
		CompanyName:       "Aesthetic Indians",
		SupportEmail:      " Help@Example.com ",
		SupportPhone:      "+91 98765 43210",
		Address:           "12 Lane, Jaipur, Rajasthan 302001",
		GrievanceOfficer:  "Asha Rao",
		EffectiveDate:     "2026-11-13",
		ReturnWindowDays:  7,
		RefundDays:        "5–7",
		DispatchDays:      "1–3",
		DeliveryDays:      "5–7",
		FreeShippingAbove: 1499,
		ShippingFee:       79,
	}
}

func newInfo(t *testing.T) staticInfo.StaticInfo {
	t.Helper()
	info, err := staticInfo.NewStaticInfo(validConfig())
	if err != nil {
		t.Fatalf("NewStaticInfo: %v", err)
	}
	return info
}

func pageText(page staticInfo.Page) string {
	var b strings.Builder
	b.WriteString(page.Title + "\n" + page.Summary + "\n")
	for _, s := range page.Sections {
		b.WriteString(s.Heading + "\n" + strings.Join(s.Paragraphs, "\n") + "\n" + strings.Join(s.Bullets, "\n") + "\n")
	}
	return b.String()
}

func TestListPages_OrderAndSlugs(t *testing.T) {
	pages, err := newInfo(t).ListPages(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"terms-and-conditions", "privacy-policy", "refund-and-return-policy", "shipping-policy", "contact-us"}
	if len(pages) != len(want) {
		t.Fatalf("got %d pages, want %d", len(pages), len(want))
	}
	for i, slug := range want {
		if pages[i].Slug != slug || pages[i].Title == "" || pages[i].Summary == "" {
			t.Errorf("page %d = %+v, want slug %s with title and summary", i, pages[i], slug)
		}
	}
}

func TestGetPage_RendersConfig(t *testing.T) {
	info := newInfo(t)
	for _, slug := range []string{"terms-and-conditions", "privacy-policy", "refund-and-return-policy", "shipping-policy", "contact-us"} {
		page, err := info.GetPage(context.Background(), slug)
		if err != nil {
			t.Fatalf("GetPage(%s): %v", slug, err)
		}
		text := pageText(page)
		if strings.Contains(text, "{{") || strings.Contains(text, "}}") {
			t.Errorf("%s has unrendered template markers", slug)
		}
		if page.EffectiveDate != "13 November 2026" {
			t.Errorf("%s effective date = %q", slug, page.EffectiveDate)
		}
		if !strings.Contains(text, "help@example.com") {
			t.Errorf("%s does not show the normalised support email", slug)
		}
	}
	refund, _ := info.GetPage(context.Background(), "refund-and-return-policy")
	if !strings.Contains(pageText(refund), "within 7 days of delivery") {
		t.Error("refund page does not state the 7 day window")
	}
	shipping, _ := info.GetPage(context.Background(), " Shipping-Policy ")
	text := pageText(shipping)
	if !strings.Contains(text, "₹1,499") || !strings.Contains(text, "₹79") {
		t.Errorf("shipping page missing rupee amounts:\n%s", text)
	}
}

func TestGetPage_ContactUsShowsAllDetails(t *testing.T) {
	page, err := newInfo(t).GetPage(context.Background(), "contact-us")
	if err != nil {
		t.Fatal(err)
	}
	text := pageText(page)
	for _, want := range []string{"Contact Us", "help@example.com", "+91 98765 43210", "Aesthetic Indians, 12 Lane, Jaipur, Rajasthan 302001", "Asha Rao", "48 hours"} {
		if !strings.Contains(text, want) {
			t.Errorf("contact page missing %q:\n%s", want, text)
		}
	}
}

func TestGetPage_ReturnsCopy(t *testing.T) {
	info := newInfo(t)
	page, _ := info.GetPage(context.Background(), "privacy-policy")
	page.Sections[0].Paragraphs[0] = "tampered"
	again, _ := info.GetPage(context.Background(), "privacy-policy")
	if again.Sections[0].Paragraphs[0] == "tampered" {
		t.Error("GetPage returned shared slices")
	}
}

func TestGetPage_Errors(t *testing.T) {
	info := newInfo(t)
	cases := map[string]struct {
		ctx  context.Context
		slug string
		want error
	}{
		"nil ctx":      {nil, "privacy-policy", staticInfo.ErrInvalidContext},
		"empty slug":   {context.Background(), "  ", staticInfo.ErrSlugInvalid},
		"path slug":    {context.Background(), "../etc", staticInfo.ErrSlugInvalid},
		"unknown slug": {context.Background(), "cookie-policy", staticInfo.ErrPageNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := info.GetPage(tc.ctx, tc.slug); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
	if _, err := info.ListPages(nil); !errors.Is(err, staticInfo.ErrInvalidContext) {
		t.Errorf("ListPages nil ctx err = %v", err)
	}
	if _, err := info.Contact(nil); !errors.Is(err, staticInfo.ErrInvalidContext) {
		t.Errorf("Contact nil ctx err = %v", err)
	}
}

func TestContact(t *testing.T) {
	contact, err := newInfo(t).Contact(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := staticInfo.Contact{CompanyName: "Aesthetic Indians", Email: "help@example.com", Phone: "+91 98765 43210", Address: "12 Lane, Jaipur, Rajasthan 302001", GrievanceOfficer: "Asha Rao"}
	if contact != want {
		t.Errorf("contact = %+v, want %+v", contact, want)
	}
}

func TestNewStaticInfo_ConfigErrors(t *testing.T) {
	cases := map[string]struct {
		mutate func(*staticInfo.Config)
		want   error
	}{
		"company":   {func(c *staticInfo.Config) { c.CompanyName = " " }, staticInfo.ErrCompanyNameRequired},
		"email":     {func(c *staticInfo.Config) { c.SupportEmail = "nope" }, staticInfo.ErrSupportEmailInvalid},
		"phone":     {func(c *staticInfo.Config) { c.SupportPhone = "123" }, staticInfo.ErrSupportPhoneInvalid},
		"address":   {func(c *staticInfo.Config) { c.Address = "" }, staticInfo.ErrAddressRequired},
		"grievance": {func(c *staticInfo.Config) { c.GrievanceOfficer = "" }, staticInfo.ErrGrievanceRequired},
		"date":      {func(c *staticInfo.Config) { c.EffectiveDate = "13/11/2026" }, staticInfo.ErrEffectiveDateInvalid},
		"window":    {func(c *staticInfo.Config) { c.ReturnWindowDays = 0 }, staticInfo.ErrReturnWindowInvalid},
		"terms":     {func(c *staticInfo.Config) { c.DeliveryDays = " " }, staticInfo.ErrTermsRequired},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			if _, err := staticInfo.NewStaticInfo(cfg); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestRupeesFormatting(t *testing.T) {
	for amount, want := range map[uint]string{0: "₹0", 999: "₹999", 1000: "₹1,000", 149999: "₹1,49,999", 12345678: "₹1,23,45,678"} {
		cfg := validConfig()
		cfg.FreeShippingAbove = amount
		info, err := staticInfo.NewStaticInfo(cfg)
		if err != nil {
			t.Fatal(err)
		}
		page, _ := info.GetPage(context.Background(), "shipping-policy")
		if !strings.Contains(pageText(page), "orders of "+want+" and above") {
			t.Errorf("amount %d not rendered as %s", amount, want)
		}
	}
}
