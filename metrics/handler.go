package metrics

import (
	"context"
	"net/http"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
)

// ResourceMiddleware to make sure update resource correct
func (m *Metrics) ResourceMiddleware(cluster cluster.Cluster) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(context.Background(), m.Config.GlobalTimeout)
			defer cancel()
			nodes, err := cluster.ListPodNodes(ctx, "", nil, true)
			if err != nil {
				log.Errorf("[ResourceMiddleware] Get all nodes err %v", err)
			}
			for _, node := range nodes {
				m.SendNodeInfo(node)
			}
			h.ServeHTTP(w, r)
		})
	}
}
