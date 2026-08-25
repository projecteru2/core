package common

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnzipFileRejectsPathTraversal(t *testing.T) {
	dest := t.TempDir()
	outside := filepath.Join(filepath.Dir(dest), "escaped.txt")

	err := unzipFile(bytes.NewReader(zipOf(t, map[string]string{"../escaped.txt": "pwned"})), dest)
	assert.Error(t, err, "a traversing entry must be rejected")
	assert.NoFileExists(t, outside)
}

func TestUnzipFileExtractsEntries(t *testing.T) {
	dest := t.TempDir()
	require.NoError(t, unzipFile(bytes.NewReader(zipOf(t, map[string]string{"a.txt": "one", "b.txt": "two"})), dest))

	for name, want := range map[string]string{"a.txt": "one", "b.txt": "two"} {
		got, err := os.ReadFile(filepath.Join(dest, name))
		require.NoError(t, err)
		assert.Equal(t, want, string(got))
	}
}

func TestUnzipFileClosesEveryEntry(t *testing.T) {
	before, ok := openFDs()
	if !ok {
		t.Skip("cannot count open file descriptors on this platform")
	}

	entries := map[string]string{}
	for i := range 200 {
		entries[fmt.Sprintf("entry-%03d.txt", i)] = "x"
	}
	require.NoError(t, unzipFile(bytes.NewReader(zipOf(t, entries)), t.TempDir()))

	after, _ := openFDs()
	assert.Less(t, after-before, 50, "unzipFile held %d extra descriptors open across %d entries", after-before, len(entries))
}

func zipOf(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	buf := &bytes.Buffer{}
	zw := zip.NewWriter(buf)
	for name, body := range entries {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		require.NoError(t, err)
		_, err = w.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func openFDs() (int, bool) {
	for _, dir := range []string{"/proc/self/fd", "/dev/fd"} {
		if entries, err := os.ReadDir(dir); err == nil {
			return len(entries), true
		}
	}
	return 0, false
}
