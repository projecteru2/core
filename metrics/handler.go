package metrics

import (
	"context"
	"net/http"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

const scrapeFanout = 8

// ResourceMiddleware refreshes node metrics before the wrapped handler runs; overlapping scrapes share one refresh.
func (m *Metrics) ResourceMiddleware(cluster cluster.Cluster) func(http.Handler) http.Handler {
	logger := log.WithFunc("metrics.ResourceMiddleware")
	var scrapes singleflight.Group
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _, _ = scrapes.Do("refresh", func() (any, error) {
				ctx, cancel := context.WithTimeout(r.Context(), m.Config.GlobalTimeout)
				defer cancel()
				nodes, err := cluster.ListPodNodes(ctx, &types.ListNodesOptions{All: true})
				if err != nil {
					logger.Error(ctx, err, "failed to list nodes")
					return nil, err
				}
				var g errgroup.Group
				g.SetLimit(scrapeFanout)
				for node := range nodes {
					g.Go(func() error {
						m.SendPodNodeStatus(ctx, node)
						m.SendNodeMetrics(ctx, node)
						return nil
					})
				}
				return nil, g.Wait()
			})
			h.ServeHTTP(w, r)
		})
	}
}
