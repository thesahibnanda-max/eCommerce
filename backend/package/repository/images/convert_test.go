package imagesRepository

import (
	"image"
	"image/color"
	"testing"
)

func TestApplyOrientation(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	mark := color.NRGBA{R: 255, A: 255}
	src.SetNRGBA(0, 0, mark)

	tests := []struct {
		orientation int
		w, h        int
		x, y        int
	}{
		{1, 3, 2, 0, 0},
		{2, 3, 2, 2, 0},
		{3, 3, 2, 2, 1},
		{4, 3, 2, 0, 1},
		{5, 2, 3, 0, 0},
		{6, 2, 3, 1, 0},
		{7, 2, 3, 1, 2},
		{8, 2, 3, 0, 2},
		{0, 3, 2, 0, 0},
		{9, 3, 2, 0, 0},
	}
	for _, tt := range tests {
		got := applyOrientation(src, tt.orientation)
		b := got.Bounds()
		if b.Dx() != tt.w || b.Dy() != tt.h {
			t.Errorf("orientation %d: size %dx%d, want %dx%d", tt.orientation, b.Dx(), b.Dy(), tt.w, tt.h)
			continue
		}
		if r, _, _, _ := got.At(tt.x, tt.y).RGBA(); r != 0xFFFF {
			t.Errorf("orientation %d: marked pixel not at (%d,%d)", tt.orientation, tt.x, tt.y)
		}
	}
}

func TestJPEGOrientationWithoutEXIF(t *testing.T) {
	if got := jpegOrientation([]byte("not exif")); got != 1 {
		t.Errorf("jpegOrientation = %d, want 1", got)
	}
}
