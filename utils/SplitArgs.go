package utils

import (
	"encoding/csv"
	"strings"
)

// SplitArgs returns an array of all arguments in string
func SplitArgs(s string) (args []string, err error) {
	if s == "" {
		return []string{""}, nil
	}
	r := csv.NewReader(strings.NewReader(s))
	r.Comma = ' ' // space
	args, err = r.Read()
	return
}
