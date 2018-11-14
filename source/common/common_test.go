package common

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	repo    = "https://github.com/VihaanVerma89/submoduleTest.git"
	rev     = "a2bf01635db316e660a2cbba80b399c21fe012cf"
	subname = "subrepo"
)

func TestSourceCode(t *testing.T) {
	g := &GitScm{}

	dname, err := ioutil.TempDir("", "source")
	assert.NoError(t, err)
	err = g.SourceCode("file:///xxxx", dname, "MASTER", false)
	assert.Error(t, err)
	err = g.SourceCode("git@xxxx", dname, "MASTER", false)
	assert.Error(t, err)
	err = g.SourceCode("gitlab@xxxx", dname, "MASTER", false)
	assert.Error(t, err)

	privFile, err := ioutil.TempFile("", "priv")
	assert.NoError(t, err)
	_, err = privFile.WriteString("privkey")
	assert.NoError(t, err)
	privFile.Close()
	pubFile, err := ioutil.TempFile("", "pub")
	_, err = pubFile.WriteString("pubkey")
	assert.NoError(t, err)
	pubFile.Close()

	g.Config.PublicKey = pubFile.Name()
	// Err when Clone, because no prikey
	err = g.SourceCode("git@xxxx", dname, "MASTER", false)
	assert.Error(t, err)
	g.Config.PrivateKey = privFile.Name()
	// Err when Clone, because key is fake and url is fake too
	err = g.SourceCode("git@xxxx", dname, "MASTER", false)
	assert.Error(t, err)

	err = g.SourceCode(repo, dname, "NOTHING", false)
	assert.Error(t, err)
	os.RemoveAll(dname)
	os.Mkdir(dname, 0755)
	err = g.SourceCode(repo, dname, rev, true)
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
	err = g.Artifact("invaildurl", savedDir)
	assert.Error(t, err)
	// no header
	err = g.Artifact(testServer.URL, savedDir)
	assert.Error(t, err)
	// vaild
	g.AuthHeaders = map[string]string{"TEST": authValue}
	err = g.Artifact(testServer.URL, savedDir)
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
