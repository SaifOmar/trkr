//go:build dev

package server

import "net/http"

func staticFS() http.FileSystem {
	return http.Dir("./server/static")
}
