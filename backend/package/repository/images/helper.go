package imagesRepository

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"github.com/gen2brain/avif"
	"github.com/rwcarlsen/goexif/exif"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
	"golang.org/x/image/draw"
	"golang.org/x/sync/errgroup"
)

func (r *repository) warmUp() error {
	img := image.NewNRGBA(image.Rect(0, 0, warmupSize, warmupSize))
	var sink discard
	if err := avif.Encode(&sink, img, r.encoder.options); err != nil {
		return fmt.Errorf("imagesRepository: avif encoder unavailable: %w", err)
	}
	return nil
}

type upload struct {
	hash     string
	source   Model
	existing string
	saved    Model
}

func (r *repository) pendingUploads(images []Model, hashes []string) ([]*upload, error) {
	jobs := make([]*upload, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for i, hash := range hashes {
		if _, dup := seen[hash]; dup {
			continue
		}
		seen[hash] = struct{}{}
		jobs = append(jobs, &upload{hash: hash, source: images[i]})
	}
	if err := r.db.View(func(tx *bolt.Tx) error {
		byHash := tx.Bucket(bucketImageByHash)
		for _, job := range jobs {
			if v := byHash.Get([]byte(job.hash)); v != nil {
				job.existing = string(v)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *repository) encodeAll(ctx context.Context, jobs []*upload) error {
	g, gctx := errgroup.WithContext(ctx)
	now, err := utils.TimeNow()
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job.existing != "" {
			continue
		}
		g.Go(func() error {
			select {
			case r.slots <- struct{}{}:
			case <-gctx.Done():
				return fmt.Errorf("%w: %w", ErrCtxInvalid, gctx.Err())
			}
			out, err := r.encoder.convert(job.source.ImageContent)
			<-r.slots
			if err != nil {
				return err
			}
			job.saved = Model{
				ID:           utils.NewULID(),
				ImageContent: out.content,
				ContentType:  contentTypeAVIF,
				Width:        out.width,
				Height:       out.height,
				Size:         len(out.content),
				OriginalSize: len(job.source.ImageContent),
				CreatedAt:    now,
				UpdatedAt:    now,
				Metadata:     job.source.Metadata,
			}
			job.saved.FileName = avifFileName(job.source.FileName, job.saved.ID)
			if job.saved.Metadata == nil {
				job.saved.Metadata = make(map[string]any)
			}
			return nil
		})
	}
	return g.Wait()
}

func (r *repository) storeAll(ctx context.Context, jobs []*upload) (map[string]string, error) {
	stored := make(map[string]string, len(jobs))
	metas := make(map[string]string, len(jobs))
	for _, job := range jobs {
		if job.existing != "" {
			stored[job.hash] = job.existing
			continue
		}
		meta, err := utils.FromObjectToStringJSON(withoutContent(job.saved))
		if err != nil {
			return nil, fmt.Errorf("imagesRepository: encode image %s: %w", job.saved.ID, err)
		}
		metas[job.hash] = meta
	}
	if len(metas) == 0 {
		return stored, nil
	}
	err := r.db.Update(func(tx *bolt.Tx) error {
		if err := utils.CheckContext(ctx); err != nil {
			return err
		}
		byHash := tx.Bucket(bucketImageByHash)
		for _, job := range jobs {
			if job.existing != "" {
				continue
			}
			if id := byHash.Get([]byte(job.hash)); id != nil {
				stored[job.hash] = string(id)
				continue
			}
			id := []byte(job.saved.ID)
			if err := tx.Bucket(bucketImageContent).Put(id, job.saved.ImageContent); err != nil {
				return err
			}
			if err := tx.Bucket(bucketImageMeta).Put(id, []byte(metas[job.hash])); err != nil {
				return err
			}
			if err := tx.Bucket(bucketHashByImage).Put(id, []byte(job.hash)); err != nil {
				return err
			}
			if err := byHash.Put([]byte(job.hash), id); err != nil {
				return err
			}
			stored[job.hash] = job.saved.ID
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func deleteInTx(tx *bolt.Tx, id string) error {
	key := []byte(id)
	if tx.Bucket(bucketImageMeta).Get(key) == nil {
		return ErrImageNotFound
	}
	hashByImage := tx.Bucket(bucketHashByImage)
	if hash := hashByImage.Get(key); hash != nil {
		if err := tx.Bucket(bucketImageByHash).Delete(hash); err != nil {
			return err
		}
		if err := hashByImage.Delete(key); err != nil {
			return err
		}
	}
	if err := tx.Bucket(bucketImageContent).Delete(key); err != nil {
		return err
	}
	return tx.Bucket(bucketImageMeta).Delete(key)
}

func (r *repository) get(id string, withContent bool) (Model, error) {
	var out Model
	err := r.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket(bucketImageMeta).Get([]byte(id))
		if raw == nil {
			return ErrImageNotFound
		}
		m, err := utils.FromStringToObjectJSON[Model](string(raw))
		if err != nil {
			return fmt.Errorf("imagesRepository: decode image %s: %w", id, err)
		}
		if withContent {
			content := tx.Bucket(bucketImageContent).Get([]byte(id))
			if content == nil {
				return fmt.Errorf("imagesRepository: content missing for image %s", id)
			}
			m.ImageContent = append([]byte(nil), content...)
		}
		if m.Metadata == nil {
			m.Metadata = make(map[string]any)
		}
		out = m
		return nil
	})
	if err != nil {
		return Model{}, err
	}
	return out, nil
}

func withoutContent(m Model) Model {
	m.ImageContent = nil
	return m
}

func avifFileName(name, id string) string {
	base := filepath.Base(strings.ReplaceAll(strings.TrimSpace(name), "\\", "/"))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == "/" {
		base = id
	}
	return base + extensionAVIF
}

func (e encoder) convert(raw []byte) (converted, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return converted{}, fmt.Errorf("%w: %w", ErrImageUnsupported, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return converted{}, ErrImageUnsupported
	}
	if cfg.Width*cfg.Height > e.maxPixels {
		return converted{}, ErrImageTooManyPixels
	}

	if format == formatAVIF && cfg.Width <= e.maxDimension && cfg.Height <= e.maxDimension {
		return converted{content: raw, width: cfg.Width, height: cfg.Height}, nil
	}

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return converted{}, fmt.Errorf("%w: %w", ErrImageUnsupported, err)
	}
	if format == formatJPEG {
		img = applyOrientation(img, jpegOrientation(raw))
	}
	img = e.fit(img)

	var out bytes.Buffer
	if err := avif.Encode(&out, img, e.options); err != nil {
		return converted{}, fmt.Errorf("imagesRepository: encode avif: %w", err)
	}
	b := img.Bounds()
	return converted{content: out.Bytes(), width: b.Dx(), height: b.Dy()}, nil
}

func (e encoder) fit(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	longest := max(w, h)
	if longest <= e.maxDimension {
		return img
	}
	nw := max(1, w*e.maxDimension/longest)
	nh := max(1, h*e.maxDimension/longest)
	dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

func jpegOrientation(raw []byte) int {
	x, err := exif.Decode(bytes.NewReader(raw))
	if err != nil {
		return 1
	}
	tag, err := x.Get(exif.Orientation)
	if err != nil {
		return 1
	}
	v, err := tag.Int(0)
	if err != nil || v < 1 || v > 8 {
		return 1
	}
	return v
}

func applyOrientation(img image.Image, orientation int) image.Image {
	if orientation <= 1 || orientation > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	swap := orientation >= 5
	dw, dh := w, h
	if swap {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch orientation {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
