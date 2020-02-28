package systemd

import (
	"fmt"
	"path/filepath"
)

const (
	eruSystemdUnitPath = `/usr/local/lib/systemd/system/`
)

func getUnitFilename(ID string) string {
	basename := fmt.Sprintf("%s.service", ID)
	return filepath.Join(eruSystemdUnitPath, basename)
}
