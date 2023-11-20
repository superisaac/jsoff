package jsoff

import (
	"regexp"
	"strings"
)

// method names
func IsMethod(name string) bool {
	matched, _ := regexp.MatchString(`^[0-9a-zA-Z\-\_\:\+\.]+$`, name)
	return matched
}

func IsPublicMethod(name string) bool {
	if strings.HasPrefix(name, "rpc.") {
		return false
	}
	matched, _ := regexp.MatchString(`^[0-9a-zA-Z][0-9a-zA-Z\-\_\:\+\.\/\\\&\#]*$`, name)
	return matched
}
