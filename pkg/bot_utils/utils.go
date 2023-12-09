package botUtils

import (
	"strings"
)

func GetCmd(s string) string {

	i := strings.Index(s, "<")
	if i >= 0 {
		j := strings.Index(s, ">")
		if j >= 0 {
			return s[i+1 : j]
		}
	}
	return ""
}
