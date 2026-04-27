package main

import "strings"

func originSummary(custom []string, footgun bool) string {
	if footgun {
		return "* (footgun)"
	}
	if len(custom) == 0 {
		return "loopback only"
	}
	return "loopback + " + strings.Join(custom, ", ")
}
