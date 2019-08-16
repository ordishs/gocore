package utils

import "regexp"

// IsRegexMatch comment
func IsRegexMatch(r string, msg string) bool {
	match, _ := regexp.MatchString(r, msg)
	return match
}
