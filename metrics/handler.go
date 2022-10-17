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
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), m.Config.GlobalTimeout)
			defer cancel()
			nodes, err := cluster.ListPodNodes(ctx, &types.ListNodesOptions{All: true})
			if err != nil {
				log.Errorf(ctx, err, "[ResourceMiddleware] Get all nodes err %v", err)
			}
			for node := range nodes {
				metrics, err := m.rmgr.GetNodeMetrics(ctx, node)
				if err != nil {
					log.Errorf(ctx, err, "[ResourceMiddleware] Get metrics failed %v", err)
					continue
				}
				m.SendMetrics(metrics...)
			}
			h.ServeHTTP(w, r)
		})
	}
}
