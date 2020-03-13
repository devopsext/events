package common

import "strings"

func IsEmpty(s string) bool {
	s1 := strings.TrimSpace(s)
	return len(s1) == 0
}
