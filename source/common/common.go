package common

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/core/types"
	git "gopkg.in/libgit2/git2go.v25"
)

// GitScm is gitlab or github source code manager
type GitScm struct {
	http.Client
	Config      types.GitConfig
	AuthHeaders map[string]string
}

func certificateCheckCallback(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	return git.ErrorCode(0)
}

func gitcheck(repository, pubkey, prikey string) error {
	if !strings.HasPrefix(repository, "git@") {
		return fmt.Errorf("Only support ssh protocol(%q), use git@... ", repository)
	}
	if _, err := os.Stat(pubkey); os.IsNotExist(err) {
		return fmt.Errorf("Public Key not found(%q)", pubkey)
	}
	if _, err := os.Stat(prikey); os.IsNotExist(err) {
		return fmt.Errorf("Private Key not found(%q)", prikey)
	}

	return nil
}

// SourceCode clone code from repository into path, by revision
func (g *GitScm) SourceCode(repository, path, revision string) error {
	pubkey := g.Config.PublicKey
	prikey := g.Config.PrivateKey

	if err := gitcheck(repository, pubkey, prikey); err != nil {
		return err
	}

	credentialsCallback := func(url, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
		ret, cred := git.NewCredSshKey("git", pubkey, prikey, "")
		return git.ErrorCode(ret), &cred
	}

	cloneOpts := &git.CloneOptions{
		FetchOptions: &git.FetchOptions{
			RemoteCallbacks: git.RemoteCallbacks{
				CredentialsCallback:      credentialsCallback,
				CertificateCheckCallback: certificateCheckCallback,
			},
		},
	}

	repo, err := git.Clone(repository, path, cloneOpts)
	if err != nil {
		return err
	}

	if err := repo.CheckoutHead(nil); err != nil {
		return err
	}

	object, err := repo.RevparseSingle(revision)
	if err != nil {
		return err
	}
	defer object.Free()
	commit, err := object.AsCommit()

	return repo.ResetToCommit(commit, git.ResetHard, &git.CheckoutOpts{Strategy: git.CheckoutSafe})
}

// Artifact download the artifact to the path, then unzip it
func (g *GitScm) Artifact(artifact, path string) error {
	req, err := http.NewRequest("GET", artifact, nil)
	if err != nil {
		return err
	}

	for k, v := range g.AuthHeaders {
		req.Header.Add(k, v)
	}

	log.Debugf("Downloading artifacts from %q", artifact)
	resp, err := g.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Download artifact error %q", artifact)
	}
	log.Debugf("Download artifacts from %q finished", artifact)

	// extract files from zipfile
	log.Debugf("Extracting files from %q", artifact)
	if err := UnzipFile(resp.Body, path); err != nil {
		return err
	}
	log.Debugf("Extraction from %q done", artifact)

	return nil
}

// UnzipFile unzip a file(from resp.Body) to the spec path
func UnzipFile(body io.ReadCloser, path string) error {
	content, err := ioutil.ReadAll(body)
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return err
	}

	// extract files from zipfile
	for _, f := range reader.File {
		zipped, err := f.Open()
		if err != nil {
			return err
		}

		defer zipped.Close()

		p := filepath.Join(path, f.Name)

		if f.FileInfo().IsDir() {
			os.MkdirAll(p, f.Mode())
			continue
		}

		writer, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE, f.Mode())
		if err != nil {
			return err
		}

		defer writer.Close()
		if _, err = io.Copy(writer, zipped); err != nil {
			return err
		}
	}
	return nil
}

// ZipFiles compresses one or many files into a single zip archive file
func ZipFiles(filename string, files []string) error {

	newfile, err := os.Create(filename)
	if err != nil {
		return err
	}
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

// Security remove the .git folder
func (g *GitScm) Security(path string) error {
	return os.RemoveAll(filepath.Join(path, ".git"))
}
