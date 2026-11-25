package frontend

import (
	"embed"
	"io/fs"
)

const staticDirectory = "staticFiles"

//go:generate npm --prefix web ci
//go:generate npm --prefix web run build

//go:embed all:staticFiles
var embeddedFiles embed.FS

func StaticFiles() (fs.FS, error) {
	return fs.Sub(embeddedFiles, staticDirectory)
}
