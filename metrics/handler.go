package metrics

import (
	"context"
	"net/http"
	"sync"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// ResourceMiddleware refreshes node metrics before the wrapped handler runs.
func (m *Metrics) ResourceMiddleware(cluster cluster.Cluster) func(http.Handler) http.Handler {
	logger := log.WithFunc("metrics.ResourceMiddleware")
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), m.Config.GlobalTimeout)
			defer cancel()
			nodes, err := cluster.ListPodNodes(ctx, &types.ListNodesOptions{All: true})
			if err != nil {
				logger.Error(ctx, err, "failed to list nodes")
				h.ServeHTTP(w, r)
				return
			}
			wg := &sync.WaitGroup{}
			for node := range nodes {
				wg.Go(func() {
					m.SendPodNodeStatus(ctx, node)
					m.SendNodeMetrics(ctx, node)
				})
			}
			wg.Wait()

			h.ServeHTTP(w, r)
		})
	}
}
