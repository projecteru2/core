package utils

import (
	"io/fs"
	"os"
	"path/filepath"
)

const executablePerm = 0o111

// ListAllExecutableFiles returns the executable files directly under basedir, not recursing.
func ListAllExecutableFiles(basedir string) ([]string, error) {
	entries, err := os.ReadDir(basedir)
	if err != nil {
		return nil, err
	}
	files := []string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if isExecutable(info.Mode().Perm()) {
			files = append(files, filepath.Join(basedir, entry.Name()))
		}
	}
	return files, nil
}

func isExecutable(perm fs.FileMode) bool {
	return perm&executablePerm != 0
}
