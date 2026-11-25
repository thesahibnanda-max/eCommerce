package staticInfoService_test

import (
	"context"
	"errors"
	"testing"

	staticInfoService "github.com/thesahibnanda-max/eCommerce/backend/package/services/static_info"
	staticInfo "github.com/thesahibnanda-max/eCommerce/backend/package/static_info"
)

func newService(t *testing.T) staticInfoService.Service {
	t.Helper()
	info, err := staticInfo.NewStaticInfo(staticInfo.Config{
		CompanyName:       "Aesthetic Indians",
		SupportEmail:      "help@example.com",
		SupportPhone:      "+919876543210",
		Address:           "Jaipur, Rajasthan",
		GrievanceOfficer:  "Asha Rao",
		EffectiveDate:     "2026-11-13",
		ReturnWindowDays:  7,
		RefundDays:        "5–7",
		DispatchDays:      "1–3",
		DeliveryDays:      "5–7",
		FreeShippingAbove: 999,
		ShippingFee:       79,
	})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := staticInfoService.NewService(info)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestNewService_RequiresInfo(t *testing.T) {
	if _, err := staticInfoService.NewService(nil); !errors.Is(err, staticInfoService.ErrStaticInfoRequired) {
		t.Errorf("err = %v, want ErrStaticInfoRequired", err)
	}
}

func TestListPages(t *testing.T) {
	resp, err := newService(t).ListPages(context.Background(), staticInfoService.ListPagesRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Pages) != 5 || resp.Contact.Email != "help@example.com" {
		t.Errorf("resp = %+v", resp)
	}
	if _, err := newService(t).ListPages(nil, staticInfoService.ListPagesRequest{}); !errors.Is(err, staticInfoService.ErrInvalidContext) {
		t.Errorf("nil ctx err = %v", err)
	}
}

func TestGetPage(t *testing.T) {
	svc := newService(t)
	resp, err := svc.GetPage(context.Background(), staticInfoService.GetPageRequest{Slug: "shipping-policy"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Page.Slug != "shipping-policy" || resp.Page.Title != "Shipping Policy" || resp.Contact.GrievanceOfficer != "Asha Rao" {
		t.Errorf("resp = %+v", resp)
	}
	cases := map[string]struct {
		ctx  context.Context
		slug string
		want error
	}{
		"nil ctx": {nil, "shipping-policy", staticInfoService.ErrInvalidContext},
		"invalid": {context.Background(), "a/b", staticInfoService.ErrSlugInvalid},
		"missing": {context.Background(), "nope", staticInfoService.ErrPageNotFound},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.GetPage(tc.ctx, staticInfoService.GetPageRequest{Slug: tc.slug}); !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}
