package imagesRepository_test

import (
	"errors"
	"testing"

	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

func TestCreateImages_SavesAllInOrder(t *testing.T) {
	repo := newRepo(t)
	a := encodePNG(t, gradient(40, 30, false))
	b := encodeJPEG(t, gradient(20, 60, false))
	got, err := repo.CreateImages(bg, []imagesRepository.Model{
		{ImageContent: a, FileName: "a.png"},
		{ImageContent: b, FileName: "b.jpg"},
	})
	if err != nil {
		t.Fatalf("CreateImages: %v", err)
	}
	if len(got) != 2 || got[0].FileName != "a.avif" || got[1].FileName != "b.avif" {
		t.Fatalf("got %d images, names %q %q", len(got), got[0].FileName, got[1].FileName)
	}
	if got[0].Width != 40 || got[1].Height != 60 {
		t.Fatalf("dimensions = %dx%d, %dx%d", got[0].Width, got[0].Height, got[1].Width, got[1].Height)
	}
	for _, img := range got {
		stored, err := repo.GetImageByID(bg, img.ID)
		if err != nil || len(stored.ImageContent) == 0 {
			t.Fatalf("GetImageByID(%s) = %v", img.ID, err)
		}
	}
}

func TestCreateImages_DeduplicatesWithinBatchAndAgainstStore(t *testing.T) {
	repo := newRepo(t)
	stored, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodePNG(t, gradient(10, 10, false))})
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	same := encodePNG(t, gradient(10, 10, false))
	other := encodePNG(t, gradient(12, 12, false))
	got, err := repo.CreateImages(bg, []imagesRepository.Model{
		{ImageContent: other},
		{ImageContent: same},
		{ImageContent: other},
	})
	if err != nil {
		t.Fatalf("CreateImages: %v", err)
	}
	if got[0].ID != got[2].ID {
		t.Fatalf("identical images in batch got ids %s and %s", got[0].ID, got[2].ID)
	}
	if got[1].ID != stored.ID {
		t.Fatalf("already stored image got id %s, want %s", got[1].ID, stored.ID)
	}
}

func TestCreateImages_RejectsWholeBatch(t *testing.T) {
	tests := []struct {
		name    string
		batch   [][]byte
		wantErr error
	}{
		{"empty batch", nil, imagesRepository.ErrBatchEmpty},
		{"corrupt item", [][]byte{nil, []byte("not an image")}, imagesRepository.ErrImageUnsupported},
		{"empty item", [][]byte{nil, {}}, imagesRepository.ErrImageEmpty},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := newRepo(t)
			valid := encodePNG(t, gradient(16, 16, false))
			batch := make([]imagesRepository.Model, len(tc.batch))
			for i, content := range tc.batch {
				if content == nil {
					content = valid
				}
				batch[i] = imagesRepository.Model{ImageContent: content}
			}
			if _, err := repo.CreateImages(bg, batch); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestDeleteImages(t *testing.T) {
	repo := newRepo(t)
	created, err := repo.CreateImages(bg, []imagesRepository.Model{
		{ImageContent: encodePNG(t, gradient(8, 8, false))},
		{ImageContent: encodePNG(t, gradient(9, 9, false))},
	})
	if err != nil {
		t.Fatalf("CreateImages: %v", err)
	}
	a, b := created[0].ID, created[1].ID

	tests := []struct {
		name    string
		ids     []string
		wantErr error
	}{
		{"empty batch", nil, imagesRepository.ErrBatchEmpty},
		{"invalid id", []string{a, "bad"}, imagesRepository.ErrIDInvalid},
		{"duplicate id", []string{a, a}, imagesRepository.ErrImageDuplicateInBatch},
		{"missing id", []string{a, utils.NewULID()}, imagesRepository.ErrImageNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := repo.DeleteImages(bg, tc.ids); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if _, err := repo.GetImageInfoByID(bg, a); err != nil {
				t.Fatalf("image a deleted by failed batch: %v", err)
			}
		})
	}

	if err := repo.DeleteImages(bg, []string{a, b}); err != nil {
		t.Fatalf("DeleteImages: %v", err)
	}
	for _, id := range []string{a, b} {
		if _, err := repo.GetImageInfoByID(bg, id); !errors.Is(err, imagesRepository.ErrImageNotFound) {
			t.Fatalf("image %s still present: %v", id, err)
		}
	}
	recreated, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodePNG(t, gradient(8, 8, false))})
	if err != nil || recreated.ID == a {
		t.Fatalf("hash index not cleared: id %s, err %v", recreated.ID, err)
	}
}
