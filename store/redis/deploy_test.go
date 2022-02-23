package redis

import (
	"context"
	"path/filepath"

	"github.com/projecteru2/core/types"
)

func (s *RediaronTestSuite) TestDeploy() {
	ctx := context.Background()
	opts := &types.DeployOptions{
		Name:         "app",
		Entrypoint:   &types.Entrypoint{Name: "entry"},
		ProcessIdent: "abc",
	}

	// no workload deployed
	nodeCount, err := s.rediaron.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	s.NoError(err)
	s.Equal(len(nodeCount), 0)
	// have workloads
	key := filepath.Join(workloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id1")
	_, err = s.rediaron.cli.Set(ctx, key, "", 0).Result()
	s.NoError(err)
	key = filepath.Join(workloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id2")
	s.NoError(err)
	_, err = s.rediaron.cli.Set(ctx, key, "", 0).Result()
	s.NoError(err)
	nodeCount, err = s.rediaron.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	s.NoError(err)
	s.Equal(nodeCount["node"], 2)
}
