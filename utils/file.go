package utils

import (
	"io/fs"
	"path/filepath"
)

const executablePerm = 0111

// ListAllExecutableFiles returns all the executable files in the given path
func ListAllExecutableFiles(basedir string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(basedir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != basedir {
			return filepath.SkipDir
		}
		if !info.IsDir() && isExecutable(info.Mode().Perm()) {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func ListAllShareLibFiles(basedir string) ([]string, error) {
	files := []string{}
	err := filepath.Walk(basedir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != basedir {
			return filepath.SkipDir
		}
		if !info.IsDir() && filepath.Ext(path) == ".so" {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

func isExecutable(perm fs.FileMode) bool {
	return perm&executablePerm == executablePerm
}
