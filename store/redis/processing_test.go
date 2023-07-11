package redis

import (
	"context"

	"github.com/projecteru2/core/types"
)

func (s *RediaronTestSuite) TestProcessing() {
	ctx := context.Background()
	processing := &types.Processing{
		Appname:   "app",
		Entryname: "entry",
		Ident:     "abc",
		Nodename:  "node",
	}

	// create
	s.NoError(s.rediaron.CreateProcessing(ctx, processing, 10))
	// create again
	s.Error(s.rediaron.CreateProcessing(ctx, processing, 10))
	s.NoError(s.rediaron.AddWorkload(ctx, &types.Workload{Name: "a_b_c"}, processing))

	nodeCount, err := s.rediaron.doLoadProcessing(ctx, processing.Appname, processing.Entryname)
	s.NoError(err)
	s.Equal(nodeCount["node"], 9)
	// delete
	s.NoError(s.rediaron.DeleteProcessing(ctx, processing))
}
