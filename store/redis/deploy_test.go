package redis

import (
	"path/filepath"

	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
)

func (s *RediaronTestSuite) TestDeploy() {
	ctx := s.T().Context()
	opts := &types.DeployOptions{
		Name:         "app",
		Entrypoint:   &types.Entrypoint{Name: "entry"},
		ProcessIdent: "abc",
		NodeFilter:   &types.NodeFilter{},
	}

	nodeCount, err := s.rediaron.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	s.NoError(err)
	s.Equal(len(nodeCount), 0)
	key := filepath.Join(common.WorkloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id1")
	_, err = s.rediaron.cli.Set(ctx, key, "", 0).Result()
	s.NoError(err)
	key = filepath.Join(common.WorkloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id2")
	s.NoError(err)
	_, err = s.rediaron.cli.Set(ctx, key, "", 0).Result()
	s.NoError(err)
	nodeCount, err = s.rediaron.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	s.NoError(err)
	s.Equal(nodeCount["node"], 2)
}
