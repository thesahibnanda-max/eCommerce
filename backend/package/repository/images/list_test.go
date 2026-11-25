package imagesRepository_test

import (
	"errors"
	"testing"

	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
)

func TestListImages(t *testing.T) {
	repo := newRepo(t)
	first, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodePNG(t, gradient(20, 20, false)), FileName: "a.png"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	second, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodePNG(t, gradient(30, 30, false)), FileName: "b.png"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	images, total, err := repo.ListImages(bg, 0, 10)
	if err != nil || total != 2 || len(images) != 2 || images[0].ID != second.ID || images[1].ID != first.ID || len(images[0].ImageContent) != 0 {
		t.Fatalf("list: %v total=%d", err, total)
	}
	if _, _, err := repo.ListImages(bg, -1, 10); !errors.Is(err, imagesRepository.ErrOffsetInvalid) {
		t.Fatalf("offset: %v", err)
	}
}
