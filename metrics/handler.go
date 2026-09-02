package metrics

import (
	"context"
	"net/http"

<<<<<<< HEAD
	"golang.org/x/sync/errgroup"
=======
>>>>>>> 5350685c
	"golang.org/x/sync/singleflight"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

<<<<<<< HEAD
const scrapeFanout = 8

// ResourceMiddleware refreshes node metrics before the wrapped handler runs; overlapping scrapes share one refresh, which outlives the scrape that started it.
func (m *Metrics) ResourceMiddleware(cluster cluster.Cluster) func(http.Handler) http.Handler {
=======
// ResourceMiddleware refreshes node metrics before the wrapped handler runs; overlapping scrapes share one refresh, which runs under the server's context.
func (m *Metrics) ResourceMiddleware(ctx context.Context, cluster cluster.Cluster) func(http.Handler) http.Handler {
>>>>>>> 5350685c
	logger := log.WithFunc("metrics.ResourceMiddleware")
	var scrapes singleflight.Group
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			refreshed := scrapes.DoChan("refresh", func() (any, error) {
<<<<<<< HEAD
				ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), m.Config.GlobalTimeout)
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
=======
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
>>>>>>> 5350685c
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
