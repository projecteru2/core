package metrics

import (
	"context"
	"net/http"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// ResourceMiddleware to make sure update resource correct
func (m *Metrics) ResourceMiddleware(cluster cluster.Cluster) func(http.Handler) http.Handler {
	logger := log.WithFunc("metrics.ResourceMiddleware")
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), m.Config.GlobalTimeout)
			defer cancel()
			nodes, err := cluster.ListPodNodes(ctx, &types.ListNodesOptions{All: true})
			if err != nil {
				logger.Error(ctx, err, "Get all nodes err")
			}
			activeNodes := make(map[string]*types.Node, 0)
			for node := range nodes {
				metrics, err := m.rmgr.GetNodeMetrics(ctx, node)
				if err != nil {
					logger.Error(ctx, err, "Get metrics failed")
					continue
				}
				activeNodes[node.Name] = node
				m.SendMetrics(ctx, metrics...)
			}
			m.DeleteInactiveNodesWithCache(ctx, activeNodes)
			h.ServeHTTP(w, r)
		})
	}
}
