package godm

import (
	"net/http"
)

func index(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "static/index.html")
}

func upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}
