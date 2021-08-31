package ordeal

func requestCombinations(reqs map[string][]interface{}) <-chan map[string]interface{} {
	ch := make(chan map[string]interface{})

	go func() {
		defer close(ch)
		req := map[string]interface{}{}
		for k, v := range reqs {
			req[k] = v[0]
		}
		ch <- req
	}()
	return ch
}
