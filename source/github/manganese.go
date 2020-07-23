package github

import (
	"fmt"

	"github.com/projecteru2/core/source/common"
	"github.com/projecteru2/core/types"
)

// New new a github obj
func New(config types.Config) (*common.GitScm, error) {
	gitConfig := config.Git
	token := fmt.Sprintf("token %s", gitConfig.Token)
	authHeaders := map[string]string{"Authorization": token}
	return common.NewGitScm(gitConfig, authHeaders)
}
