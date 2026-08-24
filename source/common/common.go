package common

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh"
)

// GitScm is the GitHub/GitLab source code manager.
type GitScm struct {
	http.Client
	Config      types.GitConfig
	AuthHeaders map[string]string

	keyBytes []byte
}

// NewGitScm builds a GitScm from config.
func NewGitScm(config types.GitConfig, authHeaders map[string]string) (*GitScm, error) {
	b, err := os.ReadFile(config.PrivateKey)
	return &GitScm{
		Config:      config,
		AuthHeaders: authHeaders,
		keyBytes:    b,
	}, err
}

func (g *GitScm) SourceCode(ctx context.Context, repository, path, revision string, submodule bool) error {
	var repo *gogit.Repository
	var err error
	ctx, cancel := context.WithTimeout(ctx, g.Config.CloneTimeout)
	defer cancel()
	opts := &gogit.CloneOptions{
		URL:      repository,
		Progress: io.Discard,
	}
	logger := log.WithFunc("source.common.SourceCode")

	switch {
	case strings.Contains(repository, "https://"):
		repo, err = gogit.PlainCloneContext(ctx, path, false, opts)
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
		}
		opts.Auth = auth
		repo, err = gogit.PlainCloneContext(ctx, path, false, opts)
	default:
		return types.ErrInvaildSCMType
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

	logger.Infof(ctx, "fetched %s at %s", repository, hash)

	if submodule {
		s, subErr := w.Submodules()
		if subErr != nil {
			return subErr
		}
		return s.Update(&gogit.SubmoduleUpdateOptions{Init: true, Auth: opts.Auth})
	}
	return err
}

func (g *GitScm) Artifact(ctx context.Context, artifact, path string) error {
	req, err := http.NewRequest(http.MethodGet, artifact, nil)
	if err != nil {
		return err
	}

	for k, v := range g.AuthHeaders {
		req.Header.Add(k, v)
	}

	log.WithFunc("source.common.Artifact").Infof(ctx, "downloading artifacts from %q", artifact)
	resp, err := g.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != 200 {
		return errors.Wrapf(types.ErrDownloadArtifactsFailed, "code: %d", resp.StatusCode)
	}

	return unzipFile(resp.Body, path)
}

func (g *GitScm) Security(path string) error {
	return os.RemoveAll(filepath.Join(path, ".git"))
}
