package utils

import (
	"io/fs"
	"path/filepath"
)

const executablePerm = 0o111

// ListAllExecutableFiles returns the executable files directly under basedir, not recursing.
func ListAllExecutableFiles(basedir string) ([]string, error) {
	files := []string{}
	err := filepath.WalkDir(basedir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != basedir {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if isExecutable(info.Mode().Perm()) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func isExecutable(perm fs.FileMode) bool {
	return perm&executablePerm != 0
}
