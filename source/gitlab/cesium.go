package gitlab

import (
	"gitlab.ricebook.net/platform/core/source/common"
	"gitlab.ricebook.net/platform/core/types"
)

func New(config types.Config) *common.GitScm {
	gitConfig := config.Git
	authheaders := map[string]string{}
	authheaders["PRIVATE-TOKEN"] = gitConfig.Token
	return &common.GitScm{Config: gitConfig, AuthHeaders: authheaders}
}
