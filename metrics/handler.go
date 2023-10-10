package metrics

import (
	"context"
	"net/http"

	"github.com/projecteru2/core/cluster"
	enginefactory "github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/prometheus/client_golang/prometheus"
	promClient "github.com/prometheus/client_model/go"
	"golang.org/x/exp/slices"
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
			existNodenames := getExistNodenames()
			activeNodes := map[string]*types.Node{}
			for node := range nodes {
				metrics, err := m.rmgr.GetNodeMetrics(ctx, node)
				if err != nil {
					logger.Error(ctx, err, "Get metrics failed")
					continue
				}
				activeNodes[node.Name] = node
				m.SendMetrics(ctx, metrics...)
			}
			// refresh nodes
			invalidNodenames := []string{}
			for _, nodename := range existNodenames {
				if node := activeNodes[nodename]; node != nil {
					invalidNodenames = append(invalidNodenames, nodename)
					enginefactory.RemoveEngineFromCache(ctx, node.Endpoint, node.Ca, node.Cert, node.Key)
				}
			}
			m.RemoveInvalidNodes(invalidNodenames...)
			h.ServeHTTP(w, r)
		})
	}
}

func getExistNodenames() []string {
	metrics, _ := prometheus.DefaultGatherer.Gather()
	nodenames := []string{}
	for _, metric := range metrics {
		for _, mf := range metric.GetMetric() {
			if i := slices.IndexFunc(mf.Label, func(label *promClient.LabelPair) bool {
				return label.GetName() == "nodename"
			}); i != -1 {
				nodenames = append(nodenames, mf.Label[i].GetValue())
			}
		}
	}
	return nodenames
}
