package metrics

//
//// ResourceMiddleware to make sure update resource correct
//// TODO: remove resource types
//func (m *Metrics) ResourceMiddleware(cluster cluster.Cluster) func(http.Handler) http.Handler {
//	return func(h http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			ctx, cancel := context.WithTimeout(context.TODO(), m.Config.GlobalTimeout)
//			defer cancel()
//			nodes, err := cluster.ListPodNodes(ctx, "", nil, true)
//			if err != nil {
//				log.Errorf(ctx, "[ResourceMiddleware] Get all nodes err %v", err)
//			}
//			for _, node := range nodes {
//				m.SendNodeInfo(node.Metrics())
//			}
//			h.ServeHTTP(w, r)
//		})
//	}
//}
