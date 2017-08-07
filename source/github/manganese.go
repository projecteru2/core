package github

import (
	"fmt"

	"gitlab.ricebook.net/platform/core/source/common"
	"gitlab.ricebook.net/platform/core/types"
)

func New(config types.Config) *common.GitScm {
	gitConfig := config.Git
	token := fmt.Sprintf("token %s", gitConfig.Token)
	authheaders := map[string]string{}
	authheaders["Authorization"] = token
	return &common.GitScm{Config: gitConfig, AuthHeaders: authheaders}
}
