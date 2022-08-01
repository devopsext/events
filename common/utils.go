package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/devopsext/utils"
)

func AsEventType(s string) string {
	return fmt.Sprintf("%sEvent", s)
}

func JsonMarshal(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}

func Content(s string) string {

	b, err := utils.Content(s)
	if err != nil {
		return s
	}
	return string(b)
}

func DeDotMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return in
	}
	ret := make(map[string]string, len(in))
	for key, value := range in {
		nKey := strings.ReplaceAll(key, ".", "_")
		ret[nKey] = value
	}
	return ret
}
