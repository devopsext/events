package common

import (
	"bytes"
	"encoding/json"
	"fmt"
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
