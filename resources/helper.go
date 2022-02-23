package resources

import "encoding/json"

// ToResp converts a map to a response
func ToResp(m interface{}, resp interface{}) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, resp)
}
