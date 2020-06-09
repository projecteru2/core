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

	git "github.com/libgit2/git2go/v30"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// GitScm is gitlab or github source code manager
type GitScm struct {
	http.Client
	Config      types.GitConfig
	AuthHeaders map[string]string
}

// SourceCode clone code from repository into path, by revision
func (g *GitScm) SourceCode(repository, path, revision string, submodule bool) error {
	if err := gitcheck(repository, g.Config.PublicKey, g.Config.PrivateKey); err != nil {
		return err
	}

	var repo *git.Repository
	var err error
	if strings.Contains(repository, "https://") {
		repo, err = git.Clone(repository, path, &git.CloneOptions{})
	} else {
		credentialsCallback := func(url, username string, allowedTypes git.CredType) (*git.Cred, error) {
			return git.NewCredSshKey(username, g.Config.PublicKey, g.Config.PrivateKey, "")
		}
		cloneOpts := &git.CloneOptions{
			FetchOptions: &git.FetchOptions{
				RemoteCallbacks: git.RemoteCallbacks{
					CredentialsCallback:      credentialsCallback,
					CertificateCheckCallback: certificateCheckCallback,
				},
			},
		}
		repo, err = git.Clone(repository, path, cloneOpts)
	}
	if err != nil {
		return err
	}
	defer repo.Free()

	if err := repo.CheckoutHead(nil); err != nil {
		return err
	}

	object, err := repo.RevparseSingle(revision)
	if err != nil {
		return err
	}
	defer object.Free()

	object, err = object.Peel(git.ObjectCommit)
	if err != nil {
		return err
	}

	commit, err := object.AsCommit()
	if err != nil {
		return err
	}
	defer commit.Free()

	tree, err := commit.Tree()
	if err != nil {
		return err
	}
	defer tree.Free()

	if err := repo.CheckoutTree(tree, &git.CheckoutOpts{Strategy: git.CheckoutSafe}); err != nil {
		return err
	}
	log.Infof("[SourceCode] Fetch repo %s", repository)
	log.Infof("[SourceCode] Checkout to commit %v", commit.Id())

	// Prepare submodules
	if submodule {
		repo.Submodules.Foreach(func(sub *git.Submodule, name string) int {
			sub.Init(true)
			err := sub.Update(true, &git.SubmoduleUpdateOptions{
				CheckoutOpts: &git.CheckoutOpts{
					Strategy: git.CheckoutForce | git.CheckoutUpdateSubmodules,
				},
				FetchOptions: &git.FetchOptions{},
			})
			if err != nil {
				log.Errorln(err)
			}
			return 0
		})
	}
	return nil
}

// Artifact download the artifact to the path, then unzip it
func (g *GitScm) Artifact(artifact, path string) error {
	req, err := http.NewRequest(http.MethodGet, artifact, nil)
	if err != nil {
		return err
	}

	for k, v := range g.AuthHeaders {
		req.Header.Add(k, v)
	}

	log.Infof("[Artifact] Downloading artifacts from %q", artifact)
	resp, err := g.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("Download artifact error %q, code %d", artifact, resp.StatusCode)
	}

	// extract files from zipfile
	return unzipFile(resp.Body, path)
}

// Security remove the .git folder
func (g *GitScm) Security(path string) error {
	return os.RemoveAll(filepath.Join(path, ".git"))
}

// unzipFile unzip a file(from resp.Body) to the spec path
func unzipFile(body io.ReadCloser, path string) error {
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

func certificateCheckCallback(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	return git.ErrorCode(0)
}

func gitcheck(repository, pubkey, prikey string) error {
	if strings.Contains(repository, "https://") {
		return nil
	}
	if strings.Contains(repository, "git@") || strings.Contains(repository, "gitlab@") {
		if _, err := os.Stat(pubkey); os.IsNotExist(err) {
			return fmt.Errorf("Public Key not found(%q)", pubkey)
		}
		if _, err := os.Stat(prikey); os.IsNotExist(err) {
			return fmt.Errorf("Private Key not found(%q)", prikey)
		}
		return nil
	}
	return types.ErrNotSupport
}
