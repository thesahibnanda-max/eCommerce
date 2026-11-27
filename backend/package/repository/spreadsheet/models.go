package spreadsheetRepository

import "time"

type Config struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
	FolderID     string
	TitlePrefix  string
	Timeout      time.Duration
}

type Snapshot struct {
	Database string
	TakenAt  time.Time
	Sheets   []Sheet
}

type Sheet struct {
	Name string
	Rows []Row
}

type Row struct {
	Key  string
	JSON string
}

type Result struct {
	SpreadsheetID string
	URL           string
	Sheets        int
	Rows          int
}
