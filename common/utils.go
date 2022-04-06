package common

import (
	"fmt"
)

func AsEventType(s string) string {
	return fmt.Sprintf("%sEvent", s)
}
