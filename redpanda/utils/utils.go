package utils

import "strings"

func IsNotFound(err error) bool {
	if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "404") {
		return true
	}
	return false
}
