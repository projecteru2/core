package common

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"golang.org/x/crypto/ssh"
)

// GitScm is gitlab or github source code manager
type GitScm struct {
	http.Client
	Config      types.GitConfig
	AuthHeaders map[string]string

	keyBytes []byte
}

// NewGitScm .
func NewGitScm(config types.GitConfig, authHeaders map[string]string) (*GitScm, error) {
	b, err := ioutil.ReadFile(config.PrivateKey)
	return &GitScm{
		Config:      config,
		AuthHeaders: authHeaders,
		keyBytes:    b,
	}, err
}

// SourceCode clone code from repository into path, by revision
func (g *GitScm) SourceCode(ctx context.Context, repository, path, revision string, submodule bool) error {
	var repo *gogit.Repository
	var err error
	ctx, cancel := context.WithTimeout(ctx, g.Config.CloneTimeout)
	defer cancel()
	switch {
	case strings.Contains(repository, "https://"):
		repo, err = gogit.PlainCloneContext(ctx, path, false, &gogit.CloneOptions{
			URL: repository,
		})
	case strings.Contains(repository, "git@") || strings.Contains(repository, "gitlab@"):
		signer, signErr := ssh.ParsePrivateKey(g.keyBytes)
		if signErr != nil {
			return signErr
		}
		splitRepo := strings.Split(repository, "@")
		user, parseErr := url.Parse(splitRepo[0])
		if parseErr != nil {
			return parseErr
		}
		auth := &gitssh.PublicKeys{
			User:   user.Host + user.Path,
			Signer: signer,
			HostKeyCallbackHelper: gitssh.HostKeyCallbackHelper{
				HostKeyCallback: ssh.InsecureIgnoreHostKey(), // nolint
			},
		}
		repo, err = gogit.PlainCloneContext(ctx, path, false, &gogit.CloneOptions{
			URL:      repository,
			Progress: ioutil.Discard,
			Auth:     auth,
		})
	default:
		return types.ErrNotSupport
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

	if err = w.Checkout(&gogit.CheckoutOptions{Hash: *hash}); err != nil {
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
		return s.Update(&gogit.SubmoduleUpdateOptions{Init: true})
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
