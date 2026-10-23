package cli

import "encoding/json"

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
