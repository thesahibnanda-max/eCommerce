package snapshotService

import (
	"context"
	"time"

	dumpRepository "github.com/thesahibnanda-max/eCommerce/backend/package/repository/dump"
)

type Source struct {
	Database string
	Dump     func(ctx context.Context) ([]dumpRepository.Table, error)
}

type DatabaseResult struct {
	Database string `json:"database"`
	URL      string `json:"url,omitempty"`
	Sheets   int    `json:"sheets"`
	Rows     int    `json:"rows"`
	Error    string `json:"error,omitempty"`
}

type Result struct {
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt time.Time        `json:"finished_at"`
	Databases  []DatabaseResult `json:"databases"`
	Failed     int              `json:"failed"`
}

type Status struct {
	Running bool    `json:"running"`
	Last    *Result `json:"last,omitempty"`
}
