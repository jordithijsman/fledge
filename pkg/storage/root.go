package storage

import (
	"os"
	"path"
)

var rootPath string

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return path.Join(home, ".fledge")
}

func SetRootPath(path string) {
	rootPath = path
}

func RootPath() string {
	if rootPath == "" {
		return DefaultPath()
	}
	return rootPath
}
