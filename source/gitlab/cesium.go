package gitlab

import (
	"github.com/projecteru2/core/source/common"
	"github.com/projecteru2/core/types"
)

// New new a gitlab obj
func New(config types.Config) (*common.GitScm, error) {
	gitConfig := config.Git
	authHeaders := map[string]string{"PRIVATE-TOKEN": gitConfig.Token}
	return common.NewGitScm(gitConfig, authHeaders)
}
