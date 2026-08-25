package types

import "encoding/json/v2"

// Decode round-trips in through JSON into out.
func Decode(in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}
