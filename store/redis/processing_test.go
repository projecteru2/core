package redis

import (
	"context"

	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func (s *RediaronTestSuite) TestProcessing() {
	ctx := context.Background()
	opts := &types.DeployOptions{
		Name:         "app",
		Entrypoint:   &types.Entrypoint{Name: "entry"},
		ProcessIdent: "abc",
	}

	// not exists
	s.Error(s.rediaron.UpdateProcessing(ctx, opts, "node", 8))
	// create
	s.NoError(s.rediaron.SaveProcessing(ctx, opts, "node", 10))
	// create again
	s.Error(s.rediaron.SaveProcessing(ctx, opts, "node", 10))
	// update
	s.NoError(s.rediaron.UpdateProcessing(ctx, opts, "node", 8))

	sis := []strategy.Info{{Nodename: "node"}}
	err := s.rediaron.doLoadProcessing(ctx, opts, sis)
	s.NoError(err)
	s.Equal(sis[0].Count, 8)
	// delete
	s.NoError(s.rediaron.DeleteProcessing(ctx, opts, "node"))
}
