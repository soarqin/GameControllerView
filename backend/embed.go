package main

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend
var frontendFiles embed.FS

// getFrontendFS returns a sub-filesystem rooted at the "frontend" directory.
func getFrontendFS() fs.FS {
	sub, err := fs.Sub(frontendFiles, "frontend")
	if err != nil {
		panic(err)
	}
	return sub
}
