package imagesService

import "time"

type ImageInfo struct {
	ID           string         `json:"id"`
	FileName     string         `json:"file_name"`
	ContentType  string         `json:"content_type"`
	Width        int            `json:"width"`
	Height       int            `json:"height"`
	Size         int            `json:"size"`
	OriginalSize int            `json:"original_size"`
	Metadata     map[string]any `json:"metadata"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Image struct {
	ImageInfo
	Content []byte `json:"content"`
}

type GetImageByIDRequest struct {
	ID string `json:"id"`
}

type GetImageByIDResponse struct {
	Image Image `json:"image"`
}

type GetImageInfoByIDRequest struct {
	ID string `json:"id"`
}

type GetImageInfoByIDResponse struct {
	Image ImageInfo `json:"image"`
}

type GetImagesInfoByIDsRequest struct {
	IDs []string `json:"ids"`
}

type GetImagesInfoByIDsResponse struct {
	Images []ImageInfo `json:"images"`
}
