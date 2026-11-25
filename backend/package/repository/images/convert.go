package imagesRepository

import (
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/gen2brain/avif"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	formatAVIF = "avif"
	formatJPEG = "jpeg"
)

type converted struct {
	content []byte
	width   int
	height  int
}

type encoder struct {
	maxDimension int
	maxPixels    int
	options      avif.Options
}
