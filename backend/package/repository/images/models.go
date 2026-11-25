package imagesRepository

import "time"

type Model struct {
	ID           string         `json:"id"`
	ImageContent []byte         `json:"image_content"`
	FileName     string         `json:"file_name"`
	ContentType  string         `json:"content_type"`
	Width        int            `json:"width"`
	Height       int            `json:"height"`
	Size         int            `json:"size"`
	OriginalSize int            `json:"original_size"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	Metadata     map[string]any `json:"metadata"`
}
