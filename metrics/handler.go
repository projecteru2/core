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
			podUpDownNodes := map[string][]int{}
			for node := range nodes {
				if podUpDownNodes[node.Podname] == nil {
					podUpDownNodes[node.Podname] = []int{0, 0}
				}
				if node.IsDown() {
					podUpDownNodes[node.Podname][1]++
				} else {
					podUpDownNodes[node.Podname][0]++
				}
				metrics, err := m.rmgr.GetNodeMetrics(ctx, node)
				if err != nil {
					logger.Error(ctx, err, "Get metrics failed")
					continue
				}
				m.SendMetrics(ctx, metrics...)
			}
			for podname, ud := range podUpDownNodes {
				m.SendPodUpDownNodes(ctx, podname, ud[0], ud[1])
			}

			h.ServeHTTP(w, r)
		})
	}
}
