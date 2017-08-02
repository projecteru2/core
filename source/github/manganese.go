package github

import (
	"fmt"

	"gitlab.ricebook.net/platform/core/source/common"
	"gitlab.ricebook.net/platform/core/types"
)

func New(config types.GitConfig) *common.GitScm {
	token := fmt.Sprintf("token %s", config.Token)
	authheaders := map[string]string{}
	authheaders["Authorization"] = token
	return &common.GitScm{Config: config, AuthHeaders: authheaders}
}
