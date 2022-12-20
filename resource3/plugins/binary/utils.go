package binary

import (
	"encoding/json"
)

func getArgs(req interface{}) ([]string, error) {
	args := []string{}
	data, err := json.Marshal(req)
	if err != nil {
		return args, err
	}
	args = append(args, string(data))
	return args, err
}
