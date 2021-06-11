package redis

import (
	"context"
	"path/filepath"

	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func (s *RediaronTestSuite) TestDeploy() {
	ctx := context.Background()
	opts := &types.DeployOptions{
		Name:         "app",
		Entrypoint:   &types.Entrypoint{Name: "entry"},
		ProcessIdent: "abc",
	}
	sis := []strategy.Info{
		{Nodename: "node"},
	}

	// no workload deployed
	err := s.rediaron.MakeDeployStatus(ctx, opts.Name, opts.Entrypoint.Name, sis)
	s.NoError(err)
	s.Equal(len(sis), 1)
	// have workloads
	key := filepath.Join(workloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id1")
	_, err = s.rediaron.cli.Set(ctx, key, "", 0).Result()
	s.NoError(err)
	key = filepath.Join(workloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id2")
	s.NoError(err)
	_, err = s.rediaron.cli.Set(ctx, key, "", 0).Result()
	s.NoError(err)
	err = s.rediaron.MakeDeployStatus(ctx, opts.Name, opts.Entrypoint.Name, sis)
	s.NoError(err)
	s.Equal(len(sis), 1)
	s.Equal(sis[0].Nodename, "node")
	s.Equal(sis[0].Count, 2)
}
