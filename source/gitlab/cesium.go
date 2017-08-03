package gitlab

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
	"gopkg.in/libgit2/git2go.v25"
)

type cesium struct {
	http.Client
	config types.Config
}

const gitUser = "git"

func certificateCheckCallback(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	return git.ErrorCode(0)
}

// clone code from repository into path, by revision
func (c *cesium) SourceCode(repository, path, revision string) error {
	pubkey := c.config.Git.PublicKey
	prikey := c.config.Git.PrivateKey

	if !strings.HasPrefix(repository, "git@") {
		return fmt.Errorf("Only support ssh protocol(%q), use git@...", repository)
	}
	if _, err := os.Stat(pubkey); os.IsNotExist(err) {
		return fmt.Errorf("Public Key not found(%q)", pubkey)
	}
	if _, err := os.Stat(prikey); os.IsNotExist(err) {
		return fmt.Errorf("Private Key not found(%q)", prikey)
	}

	credentialsCallback := func(url, username string, allowedTypes git.CredType) (git.ErrorCode, *git.Cred) {
		ret, cred := git.NewCredSshKey(gitUser, pubkey, prikey, "")
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
		log.Errorf("Error during Clone: %v", err.Error())
		return err
	}

	if err := repo.CheckoutHead(nil); err != nil {
		log.Errorf("Error during CheckoutHead: %v", err.Error())
		return err
	}

	object, err := repo.RevparseSingle(revision)
	if err != nil {
		log.Errorf("Error during RevparseSingle: %v", err.Error())
		return err
	}
	defer object.Free()
	commit, err := object.AsCommit()

	return repo.ResetToCommit(commit, git.ResetHard, &git.CheckoutOpts{Strategy: git.CheckoutSafe})
}

// Download artifact to path, and unzip it
// artifact is like "http://gitlab.ricebook.net/api/v3/projects/:project_id/builds/:build_id/artifacts"
func (c *cesium) Artifact(artifact, path string) error {
	req, err := http.NewRequest("GET", artifact, nil)
	if err != nil {
		return err
	}

	req.Header.Add("PRIVATE-TOKEN", c.config.Git.GitlabToken)
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Download artifact error %q", artifact)
	}

	log.Debugf("Downloading artifacts from %q", artifact)
	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	log.Debugf("Extracting files from %q", artifact)
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
	log.Debugf("Extraction from %q done", artifact)
	return nil
}

func (c *cesium) Security(path string) error {
	return os.RemoveAll(filepath.Join(path, ".git"))
}

func New(config types.Config) *cesium {
	return &cesium{config: config}
}
