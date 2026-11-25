package adminService_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	productsRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/products"
	adminService "github.com/thesahibnanda-max/eCommerce/backend/package/services/admin"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

const testAdminKey = "admin-secret-key"

var bg = context.Background()

type fixture struct {
	svc      adminService.Service
	products productsRepository.Repository
	images   imagesRepository.Repository
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	products, err := productsRepository.NewRepository([]string{dir, "products.db"}, 0o600)
	if err != nil {
		t.Fatalf("products repo: %v", err)
	}
	t.Cleanup(func() { _ = products.Close() })
	images, err := imagesRepository.NewRepository([]string{dir, "images.db"}, 0o600)
	if err != nil {
		t.Fatalf("images repo: %v", err)
	}
	t.Cleanup(func() { _ = images.Close() })
	svc, err := adminService.NewService(testAdminKey, products, images)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return fixture{svc: svc, products: products, images: images}
}

func productInput(name, sku string) adminService.ProductInput {
	return adminService.ProductInput{
		Name:       name,
		Categories: []string{"shirts"},
		Variants: []adminService.VariantInput{
			{SKU: sku, ShownPrice: 200, SellingPrice: 150, Currency: "inr", StockUnits: 3},
		},
	}
}

func pngBytes(t *testing.T, size int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / size), G: uint8(y * 255 / size), B: 90, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return b.Bytes()
}

func TestNewService(t *testing.T) {
	f := newFixture(t)
	tests := []struct {
		name     string
		key      string
		products productsRepository.Repository
		images   imagesRepository.Repository
		wantErr  error
	}{
		{"missing key", "  ", f.products, f.images, adminService.ErrAdminKeyRequired},
		{"missing products repo", testAdminKey, nil, f.images, adminService.ErrProductsRepoRequired},
		{"missing images repo", testAdminKey, f.products, nil, adminService.ErrImagesRepoRequired},
		{"ok", testAdminKey, f.products, f.images, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := adminService.NewService(tc.key, tc.products, tc.images); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestEveryMethodRequiresAdminKey(t *testing.T) {
	f := newFixture(t)
	for _, key := range []string{"", "wrong", testAdminKey + "x"} {
		calls := map[string]func() error{
			"CreateProduct": func() error {
				_, err := f.svc.CreateProduct(bg, adminService.CreateProductRequest{AdminKey: key, Product: productInput("A", "A-1")})
				return err
			},
			"CreateProducts": func() error {
				_, err := f.svc.CreateProducts(bg, adminService.CreateProductsRequest{AdminKey: key, Products: []adminService.ProductInput{productInput("A", "A-1")}})
				return err
			},
			"UpdateProduct": func() error {
				_, err := f.svc.UpdateProduct(bg, adminService.UpdateProductRequest{AdminKey: key})
				return err
			},
			"UpdateProducts": func() error {
				_, err := f.svc.UpdateProducts(bg, adminService.UpdateProductsRequest{AdminKey: key})
				return err
			},
			"CreateImage": func() error {
				_, err := f.svc.CreateImage(bg, adminService.CreateImageRequest{AdminKey: key})
				return err
			},
			"CreateImages": func() error {
				_, err := f.svc.CreateImages(bg, adminService.CreateImagesRequest{AdminKey: key})
				return err
			},
			"DeleteImage": func() error {
				_, err := f.svc.DeleteImage(bg, adminService.DeleteImageRequest{AdminKey: key})
				return err
			},
			"DeleteImages": func() error {
				_, err := f.svc.DeleteImages(bg, adminService.DeleteImagesRequest{AdminKey: key})
				return err
			},
		}
		for name, call := range calls {
			if err := call(); !errors.Is(err, adminService.ErrUnauthorized) {
				t.Errorf("%s with key %q: err = %v, want ErrUnauthorized", name, key, err)
			}
		}
	}
	if _, total, _ := f.products.ListProducts(bg, 0, 10); total != 0 {
		t.Fatalf("unauthorized calls saved %d products", total)
	}
}

func TestCanceledContext(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(bg)
	cancel()
	_, err := f.svc.CreateProduct(ctx, adminService.CreateProductRequest{AdminKey: testAdminKey, Product: productInput("A", "A-1")})
	if !errors.Is(err, adminService.ErrInvalidContext) {
		t.Fatalf("err = %v, want ErrInvalidContext", err)
	}
}

func TestBatchSizeLimits(t *testing.T) {
	f := newFixture(t)
	tooManyProducts := make([]adminService.ProductInput, 101)
	tooManyIDs := make([]string, 101)
	tooManyImages := make([]adminService.ImageInput, 101)

	tests := []struct {
		name    string
		call    func() error
		wantErr error
	}{
		{"create products empty", func() error {
			_, err := f.svc.CreateProducts(bg, adminService.CreateProductsRequest{AdminKey: testAdminKey})
			return err
		}, adminService.ErrBatchEmpty},
		{"create products too many", func() error {
			_, err := f.svc.CreateProducts(bg, adminService.CreateProductsRequest{AdminKey: testAdminKey, Products: tooManyProducts})
			return err
		}, adminService.ErrBatchTooLarge},
		{"update products empty", func() error {
			_, err := f.svc.UpdateProducts(bg, adminService.UpdateProductsRequest{AdminKey: testAdminKey})
			return err
		}, adminService.ErrBatchEmpty},
		{"create images too many", func() error {
			_, err := f.svc.CreateImages(bg, adminService.CreateImagesRequest{AdminKey: testAdminKey, Images: tooManyImages})
			return err
		}, adminService.ErrBatchTooLarge},
		{"delete images empty", func() error {
			_, err := f.svc.DeleteImages(bg, adminService.DeleteImagesRequest{AdminKey: testAdminKey})
			return err
		}, adminService.ErrBatchEmpty},
		{"delete images too many", func() error {
			_, err := f.svc.DeleteImages(bg, adminService.DeleteImagesRequest{AdminKey: testAdminKey, IDs: tooManyIDs})
			return err
		}, adminService.ErrBatchTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestCreateAndUpdateProduct(t *testing.T) {
	f := newFixture(t)
	created, err := f.svc.CreateProduct(bg, adminService.CreateProductRequest{AdminKey: testAdminKey, Product: productInput(" Red Shirt ", "RS-1")})
	if err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}
	p := created.Product
	if !utils.ValidateULID(p.ID) || p.Name != "Red Shirt" || len(p.Variants) != 1 || p.Variants[0].Currency != "INR" {
		t.Fatalf("created = %+v", p)
	}

	in := productInput("Red Shirt v2", "RS-1")
	in.Variants[0].ID = p.Variants[0].ID
	in.Variants = append(in.Variants, adminService.VariantInput{SKU: "RS-2", ShownPrice: 300, SellingPrice: 300, Currency: "INR"})
	updated, err := f.svc.UpdateProduct(bg, adminService.UpdateProductRequest{
		AdminKey: testAdminKey,
		Product:  adminService.ProductUpdateInput{ID: p.ID, ProductInput: in},
	})
	if err != nil {
		t.Fatalf("UpdateProduct: %v", err)
	}
	u := updated.Product
	if u.ID != p.ID || u.Name != "Red Shirt v2" || len(u.Variants) != 2 || u.Variants[0].ID != p.Variants[0].ID || !u.CreatedAt.Equal(p.CreatedAt) {
		t.Fatalf("updated = %+v", u)
	}

	_, err = f.svc.UpdateProduct(bg, adminService.UpdateProductRequest{
		AdminKey: testAdminKey,
		Product:  adminService.ProductUpdateInput{ID: utils.NewULID(), ProductInput: productInput("X", "X-1")},
	})
	if !errors.Is(err, adminService.ErrProductNotFound) {
		t.Fatalf("unknown id err = %v, want ErrProductNotFound", err)
	}
}

func TestCreateAndUpdateProducts_Batch(t *testing.T) {
	f := newFixture(t)
	created, err := f.svc.CreateProducts(bg, adminService.CreateProductsRequest{
		AdminKey: testAdminKey,
		Products: []adminService.ProductInput{productInput("A", "A-1"), productInput("B", "B-1")},
	})
	if err != nil {
		t.Fatalf("CreateProducts: %v", err)
	}
	if len(created.Products) != 2 || created.Products[0].Name != "A" || created.Products[1].Name != "B" {
		t.Fatalf("created = %+v", created.Products)
	}

	_, err = f.svc.CreateProducts(bg, adminService.CreateProductsRequest{
		AdminKey: testAdminKey,
		Products: []adminService.ProductInput{productInput("C", "C-1"), productInput("D", "A-1")},
	})
	if !errors.Is(err, adminService.ErrSKUAlreadyExists) {
		t.Fatalf("err = %v, want ErrSKUAlreadyExists", err)
	}
	if _, total, _ := f.products.ListProducts(bg, 0, 10); total != 2 {
		t.Fatalf("total = %d, want 2 (failed batch saved nothing)", total)
	}

	updates := make([]adminService.ProductUpdateInput, len(created.Products))
	for i, p := range created.Products {
		in := productInput(p.Name+" new", p.Variants[0].SKU)
		in.Variants[0].ID = p.Variants[0].ID
		updates[i] = adminService.ProductUpdateInput{ID: p.ID, ProductInput: in}
	}
	updated, err := f.svc.UpdateProducts(bg, adminService.UpdateProductsRequest{AdminKey: testAdminKey, Products: updates})
	if err != nil {
		t.Fatalf("UpdateProducts: %v", err)
	}
	if updated.Products[0].Name != "A new" || updated.Products[1].Name != "B new" {
		t.Fatalf("updated = %+v", updated.Products)
	}

	updates[1].ID = utils.NewULID()
	updates[0].Name = "A newer"
	if _, err := f.svc.UpdateProducts(bg, adminService.UpdateProductsRequest{AdminKey: testAdminKey, Products: updates}); !errors.Is(err, adminService.ErrProductNotFound) {
		t.Fatalf("err = %v, want ErrProductNotFound", err)
	}
	got, err := f.products.GetProductByID(bg, created.Products[0].ID)
	if err != nil || got.Name != "A new" {
		t.Fatalf("product A = %q, %v; want unchanged by failed batch", got.Name, err)
	}
}

func TestCreateAndDeleteImages(t *testing.T) {
	f := newFixture(t)
	single, err := f.svc.CreateImage(bg, adminService.CreateImageRequest{
		AdminKey: testAdminKey,
		Image:    adminService.ImageInput{FileName: "photo.png", Content: pngBytes(t, 24), Metadata: map[string]any{"alt": "photo"}},
	})
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	img := single.Image
	if !utils.ValidateULID(img.ID) || img.FileName != "photo.avif" || img.ContentType != "image/avif" || img.Width != 24 || img.Metadata["alt"] != "photo" {
		t.Fatalf("image = %+v", img)
	}

	batch, err := f.svc.CreateImages(bg, adminService.CreateImagesRequest{
		AdminKey: testAdminKey,
		Images: []adminService.ImageInput{
			{FileName: "a.png", Content: pngBytes(t, 10)},
			{FileName: "b.png", Content: pngBytes(t, 12)},
		},
	})
	if err != nil {
		t.Fatalf("CreateImages: %v", err)
	}
	if len(batch.Images) != 2 || batch.Images[0].FileName != "a.avif" || batch.Images[1].FileName != "b.avif" {
		t.Fatalf("batch = %+v", batch.Images)
	}

	_, err = f.svc.CreateImages(bg, adminService.CreateImagesRequest{
		AdminKey: testAdminKey,
		Images:   []adminService.ImageInput{{Content: pngBytes(t, 14)}, {Content: []byte("junk")}},
	})
	if !errors.Is(err, adminService.ErrImageUnsupported) {
		t.Fatalf("err = %v, want ErrImageUnsupported", err)
	}

	ids := []string{batch.Images[0].ID, batch.Images[1].ID}
	if _, err := f.svc.DeleteImages(bg, adminService.DeleteImagesRequest{AdminKey: testAdminKey, IDs: []string{ids[0], utils.NewULID()}}); !errors.Is(err, adminService.ErrImageNotFound) {
		t.Fatalf("err = %v, want ErrImageNotFound", err)
	}
	if _, err := f.images.GetImageInfoByID(bg, ids[0]); err != nil {
		t.Fatalf("failed batch deleted image: %v", err)
	}
	deleted, err := f.svc.DeleteImages(bg, adminService.DeleteImagesRequest{AdminKey: testAdminKey, IDs: ids})
	if err != nil || len(deleted.IDs) != 2 {
		t.Fatalf("DeleteImages = %+v, %v", deleted, err)
	}
	for _, id := range ids {
		if _, err := f.images.GetImageInfoByID(bg, id); !errors.Is(err, adminService.ErrImageNotFound) {
			t.Fatalf("image %s still present: %v", id, err)
		}
	}

	if _, err := f.svc.DeleteImage(bg, adminService.DeleteImageRequest{AdminKey: testAdminKey, ID: img.ID}); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	if _, err := f.svc.DeleteImage(bg, adminService.DeleteImageRequest{AdminKey: testAdminKey, ID: img.ID}); !errors.Is(err, adminService.ErrImageNotFound) {
		t.Fatalf("second delete err = %v, want ErrImageNotFound", err)
	}
	if _, err := f.svc.DeleteImage(bg, adminService.DeleteImageRequest{AdminKey: testAdminKey, ID: "bad"}); !errors.Is(err, adminService.ErrImageIDInvalid) {
		t.Fatalf("bad id err = %v, want ErrImageIDInvalid", err)
	}
}
