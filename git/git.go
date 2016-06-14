package git

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/libgit2/git2go.v23"
)

const gitUser = "git"

func certificateCheckCallback(cert *git.Certificate, valid bool, hostname string) git.ErrorCode {
	return git.ErrorCode(0)
}

// clone `repository` to `path` and checkout `revision`
// using pubkey and prikey
func CloneRepository(repository, path, revision, pubkey, prikey string) error {
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

	// below is code for v24
	// which is fucking unstable
	// -------------------------
	//
	//	tree, err := object.AsTree()
	//	if err != nil {
	//		return err
	//	}
	obj, err := object.Peel(git.ObjectTree)
	if err != nil {
		return err
	}

	tree, ok := obj.(*git.Tree)
	if !ok {
		return fmt.Errorf("git object is not a tree")
	}
	return repo.CheckoutTree(tree, &git.CheckoutOpts{Strategy: git.CheckoutSafe})
}
