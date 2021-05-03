package common

import (
	"archive/zip"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

const (
	repo    = "https://github.com/VihaanVerma89/submoduleTest.git"
	rev     = "a2bf01635db316e660a2cbba80b399c21fe012cf"
	subname = "subrepo"
	privkey = `-----BEGIN OPENSSH PRIVATE KEY-----
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
wBtN56wGIEOHeQAAAAN0aHkBAgMEBQY=
-----END OPENSSH PRIVATE KEY-----`
)

func TestSourceCode(t *testing.T) {
	privFile, err := ioutil.TempFile("", "priv")
	assert.NoError(t, err)
	_, err = privFile.WriteString(privkey)
	assert.NoError(t, err)
	privFile.Close()
	config := types.GitConfig{
		CloneTimeout: time.Second * 300,
		PrivateKey:   privFile.Name(),
	}
	g, err := NewGitScm(config, nil)
	assert.NoError(t, err)
	ctx := context.Background()

	dname, err := ioutil.TempDir("", "source")
	assert.NoError(t, err)
	err = g.SourceCode(ctx, "file:///xxxx", dname, "MASTER", false)
	assert.Error(t, err)
	err = g.SourceCode(ctx, "git@xxxx", dname, "MASTER", false)
	assert.Error(t, err)
	err = g.SourceCode(ctx, "gitlab@xxxx", dname, "MASTER", false)
	assert.Error(t, err)

	err = g.SourceCode(ctx, repo, dname, "NOTHING", false)
	assert.Error(t, err)

	os.RemoveAll(dname)
	os.Mkdir(dname, 0755)
	err = g.SourceCode(ctx, repo, dname, rev, true)
	assert.NoError(t, err)
	// auto clone submodule, so vendor can't remove by os.Remove
	subrepo := filepath.Join(dname, subname)
	err = os.Remove(subrepo)
	assert.Error(t, err)
	dotGit := filepath.Join(dname, ".git")
	_, err = os.Stat(dotGit)
	assert.NoError(t, err)
	// Security dir
	err = g.Security(dname)
	assert.NoError(t, err)
	_, err = os.Stat(dotGit)
	assert.Error(t, err)

	os.Remove(privFile.Name())
	os.Remove(privFile.Name())
	os.RemoveAll(dname)
}

func TestArtifact(t *testing.T) {
	rawString := "test"
	authValue := "test"
	origFile, err := ioutil.TempFile("", "orig")
	assert.NoError(t, err)
	origFile.WriteString(rawString)
	origFile.Close()
	zipFile, err := ioutil.TempFile("", "zip")
	assert.NoError(t, err)
	assert.NoError(t, zipFiles(zipFile, []string{origFile.Name()}))

	data, err := ioutil.ReadFile(zipFile.Name())
	assert.NoError(t, err)
	savedDir, err := ioutil.TempDir("", "saved")
	assert.NoError(t, err)

	g := &GitScm{}
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		r := req.Header.Get("TEST")
		if r != authValue {
			res.WriteHeader(400)
			return
		}
		res.Write(data)
	}))
	defer func() { testServer.Close() }()
	err = g.Artifact(context.TODO(), "invaildurl", savedDir)
	assert.Error(t, err)
	// no header
	err = g.Artifact(context.TODO(), testServer.URL, savedDir)
	assert.Error(t, err)
	// vaild
	g.AuthHeaders = map[string]string{"TEST": authValue}
	err = g.Artifact(context.TODO(), testServer.URL, savedDir)
	assert.NoError(t, err)

	fname := filepath.Join(savedDir, path.Base(origFile.Name()))
	_, err = os.Stat(fname)
	assert.NoError(t, err)
	saved, err := ioutil.ReadFile(fname)
	assert.NoError(t, err)
	assert.Equal(t, string(saved), rawString)

	os.Remove(origFile.Name())
	os.Remove(zipFile.Name())
	os.RemoveAll(savedDir)
}

func zipFiles(newfile *os.File, files []string) error {
	defer newfile.Close()

	zipWriter := zip.NewWriter(newfile)
	defer zipWriter.Close()

	// Add files to zip
	for _, file := range files {

		zipfile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer zipfile.Close()

		// Get the file information
		info, err := zipfile.Stat()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Change to deflate to gain better compression
		// see http://golang.org/pkg/archive/zip/#pkg-constants
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, zipfile)
		if err != nil {
			return err
		}
	}
	return nil
}
