package utils

import (
	"io/fs"
	"path/filepath"
)

const executablePerm = 0o111

// ListAllExecutableFiles returns the executable files directly under basedir, not recursing.
func ListAllExecutableFiles(basedir string) ([]string, error) {
	return listFiles(basedir, func(_ string, info fs.FileInfo) bool {
		return isExecutable(info.Mode().Perm())
	})
}

// ListAllSharedLibFiles returns the .so files directly under basedir, not recursing.
func ListAllSharedLibFiles(basedir string) ([]string, error) {
	return listFiles(basedir, func(path string, _ fs.FileInfo) bool {
		return filepath.Ext(path) == ".so"
	})
}

func listFiles(basedir string, match func(string, fs.FileInfo) bool) ([]string, error) {
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
		if match(path, info) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func isExecutable(perm fs.FileMode) bool {
	return perm&executablePerm != 0
}
