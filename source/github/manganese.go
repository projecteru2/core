package github

import (
	"fmt"

	"github.com/projecteru2/core/source/common"
	"github.com/projecteru2/core/types"
)

// New new a github obj
func New(config types.Config) *common.GitScm {
	gitConfig := config.Git
	token := fmt.Sprintf("token %s", gitConfig.Token)
	authheaders := map[string]string{}
	authheaders["Authorization"] = token
	return &common.GitScm{Config: gitConfig, AuthHeaders: authheaders}
}
