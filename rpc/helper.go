package rpc

import (
	"os"

	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

func cleanDumpFiles(data map[string]string) error {
	log.Debugf("[cleanDumpFiles] clean dump files %v", data)
	var err error
	for _, src := range data {
		if err = os.RemoveAll(src); err != nil {
			log.Errorf("[cleanDumpFiles] clean dump files failed %v", err)
		}
	}
	return err
}

func withDumpFiles(data map[string][]byte, f func(files map[string]string) error) error {
	files := map[string]string{}
	for path, data := range data {
		fname, err := utils.TempTarFile(path, data)
		if err != nil {
			defer func() {
				os.RemoveAll(fname)
				for p := range files {
					os.RemoveAll(p)
				}
			}()
			return err
		}
		files[path] = fname
	}
	log.Debugf("[withDumpFiles] with temp files %v", files)
	defer cleanDumpFiles(files)
	return f(files)
}
