package common

import (
	"bytes"
	"encoding/json"
	"fmt"

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
