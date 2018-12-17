package gitlab

import (
	"github.com/projecteru2/core/source/common"
	"github.com/projecteru2/core/types"
)

// New new a gitlab obj
func New(config types.Config) *common.GitScm {
	gitConfig := config.Git
	authheaders := map[string]string{}
	authheaders["PRIVATE-TOKEN"] = gitConfig.Token
	return &common.GitScm{Config: gitConfig, AuthHeaders: authheaders}
}
