package ordeal

func requestCombinations(args map[string][]interface{}) <-chan map[string]interface{} {
	ch := make(chan map[string]interface{})

	orderKeys := []string{}
	for k := range args {
		orderKeys = append(orderKeys, k)
	}

	var sendCombinations func([]string, map[string]interface{})
	sendCombinations = func(keys []string, r map[string]interface{}) {
		if len(keys) == len(orderKeys) {
			defer close(ch)
		}

		if len(keys) == 0 {
			nr := make(map[string]interface{})
			for k, v := range r {
				nr[k] = v
			}
			ch <- nr
			return
		}

		for _, v := range args[keys[0]] {
			r[keys[0]] = v
			sendCombinations(keys[1:], r)
		}
	}

	go sendCombinations(orderKeys, make(map[string]interface{}))

	return ch
}
