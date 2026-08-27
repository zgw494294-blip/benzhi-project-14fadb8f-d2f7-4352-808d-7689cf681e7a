package main

import "strings"

func validAddr(addr string) bool {
	return strings.HasPrefix(addr, "127.0.0.1:") && strings.LastIndex(addr, ":") > 0
}
