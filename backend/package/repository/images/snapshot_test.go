package imagesRepository_test

import (
	"testing"

	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
)

func TestDump_LeavesOutImageBytes(t *testing.T) {
	repo := newRepo(t)
	created, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodePNG(t, gradient(40, 40, false)), FileName: "a.png"})
	if err != nil {
		t.Fatal(err)
	}
	tables, err := repo.Dump(bg)
	if err != nil {
		t.Fatal(err)
	}
	meta := false
	for _, table := range tables {
		if table.Name == "images_content" {
			t.Fatal("image bytes were dumped")
		}
		if table.Name == "images_meta" && len(table.Rows) == 1 && table.Rows[0].Key == created.ID {
			meta = true
		}
	}
	if !meta {
		t.Fatalf("image metadata missing: %+v", tables)
	}
}
