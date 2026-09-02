package metrics

import (
	"context"
	"net/http"

	"golang.org/x/sync/singleflight"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// ResourceMiddleware refreshes node metrics before the wrapped handler runs; overlapping scrapes share one refresh, which runs under the server's context.
func (m *Metrics) ResourceMiddleware(ctx context.Context, cluster cluster.Cluster) func(http.Handler) http.Handler {
	logger := log.WithFunc("metrics.ResourceMiddleware")
	var scrapes singleflight.Group
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			refreshed := scrapes.DoChan("refresh", func() (any, error) {
				refreshCtx, cancel := context.WithTimeout(ctx, m.Config.GlobalTimeout)
				defer cancel()
				nodeCh, err := cluster.ListPodNodes(refreshCtx, &types.ListNodesOptions{All: true})
				if err != nil {
					logger.Error(refreshCtx, err, "failed to list nodes")
					return nil, err
				}
				var nodes []*types.Node
				for node := range nodeCh {
					m.SendPodNodeStatus(refreshCtx, node)
					nodes = append(nodes, node)
				}
				m.SendNodesMetrics(refreshCtx, nodes...)
				return nil, nil
			})
			select {
			case <-refreshed:
			case <-r.Context().Done():
				return
			}
			h.ServeHTTP(w, r)
		})
	}
}
