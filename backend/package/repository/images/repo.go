package imagesRepository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	dumpRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/dump"
	"image"
	"os"
	"path/filepath"
	"time"

	"github.com/gen2brain/avif"
	"github.com/thesahibnanda-max/eCommerce/backend/package/utils"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketImageMeta    = []byte("images_meta")
	bucketImageContent = []byte("images_content")
	bucketImageByHash  = []byte("images_by_hash")
	bucketHashByImage  = []byte("images_hash_by_id")
)

const (
	openTimeout     = time.Second
	contentTypeAVIF = "image/avif"
	extensionAVIF   = ".avif"
	maxInputBytes   = 25 << 20
	maxInputPixels  = 50_000_000
	maxDimension    = 2048
	avifQuality     = 70
	avifSpeed       = 8
	warmupSize      = 16
	maxEncodes      = 1
)

type Repository interface {
	CreateImage(ctx context.Context, image Model) (Model, error)
	CreateImages(ctx context.Context, images []Model) ([]Model, error)
	GetImageByID(ctx context.Context, id string) (Model, error)
	GetImageInfoByID(ctx context.Context, id string) (Model, error)
	ListImages(ctx context.Context, offset, limit int) ([]Model, int, error)
	DeleteImage(ctx context.Context, id string) error
	DeleteImages(ctx context.Context, ids []string) error
	Dump(ctx context.Context) ([]dumpRepository.Table, error)
	Close() error
}

type repository struct {
	db      *bolt.DB
	encoder encoder
	slots   chan struct{}
}

func NewRepository(path []string, permission os.FileMode) (Repository, error) {
	db, err := bolt.Open(filepath.Join(path...), permission, &bolt.Options{Timeout: openTimeout})
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range [][]byte{bucketImageMeta, bucketImageContent, bucketImageByHash, bucketHashByImage} {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		_ = db.Close()
		return nil, err
	}

	r := &repository{
		db: db,
		encoder: encoder{
			maxDimension: maxDimension,
			maxPixels:    maxInputPixels,
			options: avif.Options{
				Quality:           avifQuality,
				QualityAlpha:      avifQuality,
				Speed:             avifSpeed,
				ChromaSubsampling: image.YCbCrSubsampleRatio420,
			},
		},
		slots: make(chan struct{}, maxEncodes),
	}

	if err := r.warmUp(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return r, nil
}

func (r *repository) CreateImage(ctx context.Context, img Model) (Model, error) {
	images, err := r.CreateImages(ctx, []Model{img})
	if err != nil {
		return Model{}, err
	}
	return images[0], nil
}

func (r *repository) CreateImages(ctx context.Context, images []Model) ([]Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, ErrBatchEmpty
	}
	hashes := make([]string, len(images))
	for i, img := range images {
		if len(img.ImageContent) == 0 {
			return nil, ErrImageEmpty
		}
		if len(img.ImageContent) > maxInputBytes {
			return nil, ErrImageTooLarge
		}
		sum := sha256.Sum256(img.ImageContent)
		hashes[i] = hex.EncodeToString(sum[:])
	}

	jobs, err := r.pendingUploads(images, hashes)
	if err != nil {
		return nil, err
	}
	if err := r.encodeAll(ctx, jobs); err != nil {
		return nil, err
	}
	stored, err := r.storeAll(ctx, jobs)
	if err != nil {
		return nil, err
	}

	out := make([]Model, len(images))
	byID := make(map[string]Model, len(stored))
	for i, hash := range hashes {
		id := stored[hash]
		if m, ok := byID[id]; ok {
			out[i] = m
			continue
		}
		m, err := r.get(id, true)
		if err != nil {
			return nil, err
		}
		byID[id] = m
		out[i] = m
	}
	return out, nil
}

func (r *repository) GetImageByID(ctx context.Context, id string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(id) {
		return Model{}, ErrIDInvalid
	}
	return r.get(id, true)
}

func (r *repository) GetImageInfoByID(ctx context.Context, id string) (Model, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return Model{}, err
	}
	if !utils.ValidateULID(id) {
		return Model{}, ErrIDInvalid
	}
	return r.get(id, false)
}

func (r *repository) DeleteImage(ctx context.Context, id string) error {
	return r.DeleteImages(ctx, []string{id})
}

func (r *repository) DeleteImages(ctx context.Context, ids []string) error {
	if err := utils.CheckContext(ctx); err != nil {
		return err
	}
	if len(ids) == 0 {
		return ErrBatchEmpty
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if !utils.ValidateULID(id) {
			return ErrIDInvalid
		}
		if _, dup := seen[id]; dup {
			return ErrImageDuplicateInBatch
		}
		seen[id] = struct{}{}
	}
	return r.db.Update(func(tx *bolt.Tx) error {
		if err := utils.CheckContext(ctx); err != nil {
			return err
		}
		for _, id := range ids {
			if err := deleteInTx(tx, id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *repository) Close() error {
	return r.db.Close()
}

const maxListLimit = 100

func (r *repository) ListImages(ctx context.Context, offset, limit int) ([]Model, int, error) {
	if err := utils.CheckContext(ctx); err != nil {
		return nil, 0, err
	}
	if limit < 1 || limit > maxListLimit {
		return nil, 0, ErrLimitInvalid
	}
	if offset < 0 {
		return nil, 0, ErrOffsetInvalid
	}
	out := []Model{}
	total := 0
	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketImageMeta)
		total = bucket.Stats().KeyN
		cursor := bucket.Cursor()
		skipped := 0
		for k, v := cursor.Last(); k != nil && len(out) < limit; k, v = cursor.Prev() {
			if skipped < offset {
				skipped++
				continue
			}
			image, err := utils.FromStringToObjectJSON[Model](string(v))
			if err != nil {
				return fmt.Errorf("imagesRepository: decode image %s: %w", k, err)
			}
			if image.Metadata == nil {
				image.Metadata = make(map[string]any)
			}
			out = append(out, image)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}
