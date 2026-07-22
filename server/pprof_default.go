//go:build !dev

package server

import "net/http"

func registerPprof(mux *http.ServeMux) {}
