package metrics

import (
	"context"
	"net/http"

	"github.com/projecteru2/core/cluster"
	log "github.com/sirupsen/logrus"
)

// ResourceMiddleware to make sure update resource correct
func (m *Metrics) ResourceMiddleware(cluster cluster.Cluster) func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pods, err := cluster.ListPods(context.Background())
			if err != nil {
				log.Errorf("[ResourceMiddleware] List pods err %v", err)
				// Nothing to do here, pods will be nil
			}
			for _, pod := range pods {
				nodes, err := cluster.ListPodNodes(context.Background(), pod.Name, nil, true)
				if err != nil {
					log.Errorf("[ResourceMiddleware] List pod %s nodes err %v", pod.Name, err)
					continue
				}
				for _, node := range nodes {
					m.SendNodeInfo(node)
				}
			}
			h.ServeHTTP(w, r)
		})
	}
}
