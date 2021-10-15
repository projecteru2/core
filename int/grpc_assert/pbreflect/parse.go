package pbreflect

import (
	"path/filepath"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/pkg/errors"
)

// Parse from proto file
func Parse(filename string) (service *Service, err error) {
	dirname, basename := filepath.Dir(filename), filepath.Base(filename)
	filenames, err := protoparse.ResolveFilenames([]string{dirname}, basename)
	if err != nil {
		return
	}
	p := protoparse.Parser{
		ImportPaths:           []string{dirname},
		InferImportPaths:      false,
		IncludeSourceCodeInfo: true,
	}
	parsedFiles, err := p.ParseFiles(filenames...)
	if err != nil {
		return
	}
	if len(parsedFiles) < 1 {
		return service, errors.New("proto file not found")
	}

	service = MustNewService()
	for _, parsedFile := range parsedFiles {
		for _, svc := range parsedFile.GetServices() {
			for _, method := range svc.GetMethods() {
				service.rpcs[method.GetName()] = method
			}
		}
	}

	return
}
