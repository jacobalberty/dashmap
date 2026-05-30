//go:build !embedspa

package web

import "net/http"

// Handler returns nil when the SPA is not embedded.
// The API server skips SPA routing entirely in this build.
func Handler() http.Handler { return nil }
