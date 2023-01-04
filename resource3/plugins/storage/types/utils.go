package types

import "strings"

func getDelimiterCount(str string, delimiter int32) int {
	count := 0
	for _, c := range str {
		if c == delimiter {
			count++
		}
	}
	return count
}

func hasPrefix(path string, mount string) bool {
	// /data -> /data/xxx
	// /data -> /data
	// / -> /asdf
	mount = addSlash(mount)
	path = addSlash(path)
	return strings.HasPrefix(path, mount)
}

func addSlash(dir string) string {
	if !strings.HasSuffix(dir, "/") {
		dir += "/"
	}
	return dir
}
