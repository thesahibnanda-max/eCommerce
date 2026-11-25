package imagesRepository_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"sync"
	"testing"

	"github.com/gen2brain/avif"
	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
)

var bg = context.Background()

func newRepo(t *testing.T) imagesRepository.Repository {
	t.Helper()
	repo, err := imagesRepository.NewRepository([]string{t.TempDir(), "images.db"}, 0o600)
	if err != nil {
		t.Fatalf("NewRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func gradient(w, h int, alpha bool) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(255)
			if alpha && x < w/2 {
				a = 0
			}
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 128, A: a})
		}
	}
	return img
}

func encodeJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func withEXIFOrientation(jpg []byte, orientation uint16) []byte {
	tiff := []byte{'I', 'I', 0x2A, 0x00, 0x08, 0x00, 0x00, 0x00, 0x01, 0x00}
	entry := make([]byte, 12)
	binary.LittleEndian.PutUint16(entry[0:], 0x0112)
	binary.LittleEndian.PutUint16(entry[2:], 3)
	binary.LittleEndian.PutUint32(entry[4:], 1)
	binary.LittleEndian.PutUint16(entry[8:], orientation)
	tiff = append(tiff, entry...)
	tiff = append(tiff, 0, 0, 0, 0)

	payload := append([]byte("Exif\x00\x00"), tiff...)
	segment := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(segment[2:], uint16(len(payload)+2))
	segment = append(segment, payload...)

	out := append([]byte{}, jpg[:2]...)
	out = append(out, segment...)
	return append(out, jpg[2:]...)
}

func pngHeaderOnly(width, height uint32) []byte {
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], width)
	binary.BigEndian.PutUint32(ihdr[4:], height)
	ihdr[8], ihdr[9] = 8, 6
	chunk := append([]byte("IHDR"), ihdr...)
	out := []byte("\x89PNG\r\n\x1a\n")
	length := make([]byte, 4)
	binary.BigEndian.PutUint32(length, 13)
	out = append(out, length...)
	out = append(out, chunk...)
	crc := make([]byte, 4)
	binary.BigEndian.PutUint32(crc, crc32.ChecksumIEEE(chunk))
	return append(out, crc...)
}

func decodeAVIF(t *testing.T, content []byte) image.Image {
	t.Helper()
	img, err := avif.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("stored content is not a valid AVIF: %v", err)
	}
	return img
}

func TestCreateImage_ConvertsFormatsToAVIF(t *testing.T) {
	repo := newRepo(t)

	var gifBuf bytes.Buffer
	if err := gif.Encode(&gifBuf, gradient(48, 32, false), nil); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		raw  []byte
		w, h int
	}{
		{"jpeg", encodeJPEG(t, gradient(64, 48, false)), 64, 48},
		{"png with alpha", encodePNG(t, gradient(40, 40, true)), 40, 40},
		{"gif", gifBuf.Bytes(), 48, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: tt.raw, FileName: "photo.jpg"})
			if err != nil {
				t.Fatalf("CreateImage: %v", err)
			}
			if !utils.ValidateULID(got.ID) {
				t.Errorf("ID %q is not a ULID", got.ID)
			}
			if got.ContentType != "image/avif" {
				t.Errorf("ContentType = %q", got.ContentType)
			}
			if got.Width != tt.w || got.Height != tt.h {
				t.Errorf("size = %dx%d, want %dx%d", got.Width, got.Height, tt.w, tt.h)
			}
			if got.Size != len(got.ImageContent) || got.OriginalSize != len(tt.raw) {
				t.Errorf("Size=%d len=%d OriginalSize=%d raw=%d", got.Size, len(got.ImageContent), got.OriginalSize, len(tt.raw))
			}
			dec := decodeAVIF(t, got.ImageContent)
			if b := dec.Bounds(); b.Dx() != tt.w || b.Dy() != tt.h {
				t.Errorf("decoded size = %v", b)
			}
		})
	}
}

func TestCreateImage_KeepsTransparency(t *testing.T) {
	repo := newRepo(t)
	got, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodePNG(t, gradient(40, 40, true))})
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	dec := decodeAVIF(t, got.ImageContent)
	_, _, _, transparent := dec.At(5, 20).RGBA()
	_, _, _, opaque := dec.At(35, 20).RGBA()
	if transparent > 0x1000 || opaque < 0xF000 {
		t.Errorf("alpha lost: left=%#x right=%#x", transparent, opaque)
	}
}

func TestCreateImage_ResizesLargeImagesKeepingAspectRatio(t *testing.T) {
	repo := newRepo(t)
	raw := encodeJPEG(t, gradient(3000, 1500, false))

	got, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: raw})
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	if got.Width != 2048 || got.Height != 1024 {
		t.Errorf("size = %dx%d, want 2048x1024", got.Width, got.Height)
	}
	if got.Size >= len(raw) {
		t.Errorf("AVIF %d bytes is not smaller than JPEG input %d bytes", got.Size, len(raw))
	}

	tall, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodeJPEG(t, gradient(500, 2500, false))})
	if err != nil {
		t.Fatalf("CreateImage tall: %v", err)
	}
	if tall.Width != 409 || tall.Height != 2048 {
		t.Errorf("tall size = %dx%d, want 409x2048", tall.Width, tall.Height)
	}
}

func TestCreateImage_AppliesJPEGOrientation(t *testing.T) {
	repo := newRepo(t)
	src := gradient(60, 30, false)
	for x := 0; x < 10; x++ {
		for y := 0; y < 10; y++ {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
		}
	}

	got, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: withEXIFOrientation(encodeJPEG(t, src), 6)})
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	if got.Width != 30 || got.Height != 60 {
		t.Fatalf("size = %dx%d, want 30x60 after 90° rotation", got.Width, got.Height)
	}
	r, g, _, _ := decodeAVIF(t, got.ImageContent).At(25, 4).RGBA()
	if r < 0xC000 || g > 0x6000 {
		t.Errorf("red corner not at top-right after rotation: r=%#x g=%#x", r, g)
	}
}

func TestCreateImage_AVIFWithinLimitsIsStoredAsIs(t *testing.T) {
	repo := newRepo(t)
	var b bytes.Buffer
	if err := avif.Encode(&b, gradient(32, 32, false)); err != nil {
		t.Fatal(err)
	}
	got, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: b.Bytes()})
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	if !bytes.Equal(got.ImageContent, b.Bytes()) {
		t.Error("AVIF input was re-encoded, want stored unchanged")
	}
}

func TestCreateImage_FileNameAndMetadata(t *testing.T) {
	repo := newRepo(t)
	cases := map[string]string{
		"photo.JPG":             "photo.avif",
		"  ../../etc/shoe.png ": "shoe.avif",
		`C:\Users\me\bag.webp`:  "bag.avif",
		"noext":                 "noext.avif",
	}
	i := 0
	for in, want := range cases {
		i++
		got, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodePNG(t, gradient(8+i, 8, false)), FileName: in})
		if err != nil {
			t.Fatalf("CreateImage(%q): %v", in, err)
		}
		if got.FileName != want {
			t.Errorf("FileName(%q) = %q, want %q", in, got.FileName, want)
		}
	}

	got, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodePNG(t, gradient(20, 8, false)), Metadata: map[string]any{"alt": "red shoe"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.FileName != got.ID+".avif" {
		t.Errorf("empty FileName = %q, want %s.avif", got.FileName, got.ID)
	}
	stored, _ := repo.GetImageInfoByID(bg, got.ID)
	if stored.Metadata["alt"] != "red shoe" {
		t.Errorf("Metadata = %v", stored.Metadata)
	}
}

func TestCreateImage_Errors(t *testing.T) {
	repo := newRepo(t)
	canceled, cancel := context.WithCancel(bg)
	cancel()
	valid := encodePNG(t, gradient(8, 8, false))

	tests := []struct {
		name    string
		ctx     context.Context
		raw     []byte
		wantErr error
	}{
		{"nil ctx", nil, valid, imagesRepository.ErrCtxInvalid},
		{"canceled ctx", canceled, valid, context.Canceled},
		{"empty", bg, nil, imagesRepository.ErrImageEmpty},
		{"too many bytes", bg, make([]byte, 25<<20+1), imagesRepository.ErrImageTooLarge},
		{"not an image", bg, []byte("definitely not an image"), imagesRepository.ErrImageUnsupported},
		{"truncated png", bg, valid[:len(valid)/2], imagesRepository.ErrImageUnsupported},
		{"decompression bomb", bg, pngHeaderOnly(10000, 10000), imagesRepository.ErrImageTooManyPixels},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.CreateImage(tt.ctx, imagesRepository.Model{ImageContent: tt.raw})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if got.ID != "" {
				t.Errorf("returned %+v on error", got.ID)
			}
		})
	}
}

func TestCreateImage_DeduplicatesIdenticalUploads(t *testing.T) {
	repo := newRepo(t)
	raw := encodeJPEG(t, gradient(64, 64, false))

	first, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: raw, FileName: "a.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: raw, FileName: "b.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.FileName != "a.avif" {
		t.Errorf("duplicate upload stored again: first=%s second=%s (%s)", first.ID, second.ID, second.FileName)
	}

	if err := repo.DeleteImage(bg, first.ID); err != nil {
		t.Fatal(err)
	}
	third, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: raw})
	if err != nil {
		t.Fatal(err)
	}
	if third.ID == first.ID {
		t.Error("re-upload after delete reused the deleted ID")
	}
}

func TestCreateImage_ConcurrentIdenticalUploadsStoreOnce(t *testing.T) {
	repo := newRepo(t)
	raw := encodePNG(t, gradient(48, 48, false))

	const workers = 8
	ids := make([]string, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: raw})
			if err != nil {
				t.Errorf("CreateImage: %v", err)
				return
			}
			ids[i] = got.ID
		}(i)
	}
	wg.Wait()
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("concurrent identical uploads got different IDs: %v", ids)
		}
	}
}

func TestGetAndDelete(t *testing.T) {
	repo := newRepo(t)
	created, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: encodePNG(t, gradient(16, 16, false)), FileName: "x.png"})
	if err != nil {
		t.Fatal(err)
	}

	full, err := repo.GetImageByID(bg, created.ID)
	if err != nil || !bytes.Equal(full.ImageContent, created.ImageContent) || full.FileName != "x.avif" || full.Width != 16 {
		t.Errorf("GetImageByID = %+v, %v", full.FileName, err)
	}
	if !full.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", full.CreatedAt, created.CreatedAt)
	}

	info, err := repo.GetImageInfoByID(bg, created.ID)
	if err != nil || info.ImageContent != nil || info.Size != created.Size {
		t.Errorf("GetImageInfoByID content=%d size=%d err=%v", len(info.ImageContent), info.Size, err)
	}

	full.ImageContent[0] ^= 0xFF
	again, _ := repo.GetImageByID(bg, created.ID)
	if !bytes.Equal(again.ImageContent, created.ImageContent) {
		t.Error("mutating returned content changed stored content")
	}

	if err := repo.DeleteImage(bg, created.ID); err != nil {
		t.Fatalf("DeleteImage: %v", err)
	}
	if _, err := repo.GetImageByID(bg, created.ID); !errors.Is(err, imagesRepository.ErrImageNotFound) {
		t.Errorf("after delete err = %v", err)
	}
	if err := repo.DeleteImage(bg, created.ID); !errors.Is(err, imagesRepository.ErrImageNotFound) {
		t.Errorf("double delete err = %v", err)
	}

	for _, id := range []string{"bad", ""} {
		if _, err := repo.GetImageByID(bg, id); !errors.Is(err, imagesRepository.ErrIDInvalid) {
			t.Errorf("GetImageByID(%q) err = %v", id, err)
		}
		if err := repo.DeleteImage(bg, id); !errors.Is(err, imagesRepository.ErrIDInvalid) {
			t.Errorf("DeleteImage(%q) err = %v", id, err)
		}
	}
	if _, err := repo.GetImageInfoByID(bg, utils.NewULID()); !errors.Is(err, imagesRepository.ErrImageNotFound) {
		t.Errorf("unknown id err = %v", err)
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	path := []string{t.TempDir(), "images.db"}
	repo, err := imagesRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	raw := encodePNG(t, gradient(24, 24, false))
	created, err := repo.CreateImage(bg, imagesRepository.Model{ImageContent: raw})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := imagesRepository.NewRepository(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	got, err := reopened.GetImageByID(bg, created.ID)
	if err != nil || !bytes.Equal(got.ImageContent, created.ImageContent) {
		t.Errorf("after reopen: %v", err)
	}
	dup, err := reopened.CreateImage(bg, imagesRepository.Model{ImageContent: raw})
	if err != nil || dup.ID != created.ID {
		t.Errorf("dedup index lost after reopen: %s vs %s, %v", dup.ID, created.ID, err)
	}
}
