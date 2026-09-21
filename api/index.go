// Package handler is the Vercel Go serverless entrypoint. vercel.json rewrites
// every path here; routing happens inside the application's ServeMux.
//
// It deliberately imports only the module root package (no internal/...)
// because Vercel's Go builder compiles this file in a generated wrapper.
package handler

import (
	"net/http"

	gfs "github.com/gloitel/global-freight-solutions"
)

// Handler is invoked by the Vercel Go runtime.
func Handler(w http.ResponseWriter, r *http.Request) {
	gfs.ServeServerless(w, r)
}
