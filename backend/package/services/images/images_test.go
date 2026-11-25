package imagesService_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	imagesService "github.com/thesahibnanda-max/eCommerce/backend/package/services/images"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var bg = context.Background()

type fixture struct {
	svc  imagesService.Service
	repo imagesRepository.Repository
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	repo, err := imagesRepository.NewRepository([]string{t.TempDir(), "images.db"}, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	svc, err := imagesService.NewService(repo)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return fixture{svc: svc, repo: repo}
}

func (f fixture) seed(t *testing.T, size int, name string) imagesRepository.Model {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / size), G: uint8(y * 255 / size), B: 60, A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	created, err := f.repo.CreateImage(bg, imagesRepository.Model{ImageContent: b.Bytes(), FileName: name, Metadata: map[string]any{"alt": name}})
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	return created
}

func TestNewService(t *testing.T) {
	if _, err := imagesService.NewService(nil); !errors.Is(err, imagesService.ErrImagesRepoRequired) {
		t.Fatalf("err = %v, want ErrImagesRepoRequired", err)
	}
}

func TestGetImage(t *testing.T) {
	f := newFixture(t)
	stored := f.seed(t, 20, "cat.png")

	full, err := f.svc.GetImageByID(bg, imagesService.GetImageByIDRequest{ID: " " + stored.ID + " "})
	if err != nil {
		t.Fatalf("GetImageByID: %v", err)
	}
	if full.Image.ID != stored.ID || full.Image.FileName != "cat.avif" || full.Image.ContentType != "image/avif" || full.Image.Metadata["alt"] != "cat.png" {
		t.Fatalf("image info = %+v", full.Image.ImageInfo)
	}
	if !bytes.Equal(full.Image.Content, stored.ImageContent) {
		t.Fatalf("content differs from stored bytes")
	}

	info, err := f.svc.GetImageInfoByID(bg, imagesService.GetImageInfoByIDRequest{ID: stored.ID})
	if err != nil || info.Image.ID != stored.ID || info.Image.Width != 20 || info.Image.Size != len(stored.ImageContent) {
		t.Fatalf("GetImageInfoByID = %+v, %v", info, err)
	}

	tests := []struct {
		name    string
		id      string
		wantErr error
	}{
		{"invalid id", "bad", imagesService.ErrIDInvalid},
		{"unknown id", utils.NewULID(), imagesService.ErrImageNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.svc.GetImageByID(bg, imagesService.GetImageByIDRequest{ID: tc.id}); !errors.Is(err, tc.wantErr) {
				t.Fatalf("GetImageByID err = %v, want %v", err, tc.wantErr)
			}
			if _, err := f.svc.GetImageInfoByID(bg, imagesService.GetImageInfoByIDRequest{ID: tc.id}); !errors.Is(err, tc.wantErr) {
				t.Fatalf("GetImageInfoByID err = %v, want %v", err, tc.wantErr)
			}
		})
	}

	ctx, cancel := context.WithCancel(bg)
	cancel()
	if _, err := f.svc.GetImageByID(ctx, imagesService.GetImageByIDRequest{ID: stored.ID}); !errors.Is(err, imagesService.ErrInvalidContext) {
		t.Fatalf("canceled ctx err = %v, want ErrInvalidContext", err)
	}
}

func TestGetImagesInfoByIDs(t *testing.T) {
	f := newFixture(t)
	a := f.seed(t, 10, "a.png")
	b := f.seed(t, 12, "b.png")

	res, err := f.svc.GetImagesInfoByIDs(bg, imagesService.GetImagesInfoByIDsRequest{IDs: []string{b.ID, a.ID}})
	if err != nil {
		t.Fatalf("GetImagesInfoByIDs: %v", err)
	}
	if len(res.Images) != 2 || res.Images[0].ID != b.ID || res.Images[1].ID != a.ID {
		t.Fatalf("images = %+v, want input order", res.Images)
	}

	tests := []struct {
		name    string
		ids     []string
		wantErr error
	}{
		{"empty", nil, imagesService.ErrBatchEmpty},
		{"too many", make([]string, 101), imagesService.ErrTooManyIDs},
		{"duplicate", []string{a.ID, " " + a.ID}, imagesService.ErrImageDuplicateInBatch},
		{"one missing", []string{a.ID, utils.NewULID()}, imagesService.ErrImageNotFound},
		{"one invalid", []string{a.ID, "bad"}, imagesService.ErrIDInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := f.svc.GetImagesInfoByIDs(bg, imagesService.GetImagesInfoByIDsRequest{IDs: tc.ids})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if got.Images != nil {
				t.Fatalf("got partial result %+v", got.Images)
			}
		})
	}
}
