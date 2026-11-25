package imagesService

import (
	"strings"

	imagesRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/images"
)

func normalizeIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, ErrBatchEmpty
	}
	if len(ids) > maxIDs {
		return nil, ErrTooManyIDs
	}
	out := make([]string, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for i, id := range ids {
		id = strings.TrimSpace(id)
		if _, dup := seen[id]; dup {
			return nil, ErrImageDuplicateInBatch
		}
		seen[id] = struct{}{}
		out[i] = id
	}
	return out, nil
}

func toImageInfo(m imagesRepository.Model) ImageInfo {
	return ImageInfo{
		ID:           m.ID,
		FileName:     m.FileName,
		ContentType:  m.ContentType,
		Width:        m.Width,
		Height:       m.Height,
		Size:         m.Size,
		OriginalSize: m.OriginalSize,
		Metadata:     m.Metadata,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}
