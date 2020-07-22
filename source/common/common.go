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

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
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
	var repo *gogit.Repository
	var err error
	if strings.Contains(repository, "https://") {
		repo, err = gogit.PlainClone(path, false, &gogit.CloneOptions{
			URL: repository,
		})
	} else {
		sshKey, keyErr := ioutil.ReadFile(g.Config.PrivateKey)
		if keyErr != nil {
			return keyErr
		}

		signer, signErr := ssh.ParsePrivateKey(sshKey)
		if signErr != nil {
			return signErr
		}

		splitRepo := strings.Split(repository, "@")

		auth := &gitssh.PublicKeys{
			User:   splitRepo[0],
			Signer: signer,
			HostKeyCallbackHelper: gitssh.HostKeyCallbackHelper{
				/* #nosec*/
				HostKeyCallback: ssh.InsecureIgnoreHostKey(),
			},
		}
		repo, err = gogit.PlainClone(path, false, &gogit.CloneOptions{
			URL:      repository,
			Progress: os.Stdout,
			Auth:     auth,
		})
	}

	if err != nil {
		return err
	}

	w, err := repo.Worktree()

	if err != nil {
		return err
	}

	hash, err := repo.ResolveRevision(plumbing.Revision(revision))
	if err != nil {
		return err
	}

	err = w.Checkout(&gogit.CheckoutOptions{Hash: *hash})
	if err != nil {
		return err
	}

	log.Infof("[SourceCode] Fetch repo %s", repository)
	log.Infof("[SourceCode] Checkout to commit %s", hash)

	// Prepare submodules
	if submodule {
		s, err := w.Submodules()
		if err != nil {
			return err
		}
		return s.Update(&gogit.SubmoduleUpdateOptions{
			Init: true,
		})
	}
	return err
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
func unzipFile(body io.Reader, path string) error {
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

		//  G305: File traversal when extracting zip archive
		p := filepath.Join(path, f.Name) // nolint

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(p, f.Mode())
			continue
		}

		writer, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE, f.Mode())
		if err != nil {
			return err
		}

		defer writer.Close()
		if _, err = io.Copy(writer, zipped); err != nil { // nolint
			// G110: Potential DoS vulnerability via decompression bomb
			return err
		}
	}
	return nil
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
