package godm

import (
	"net/http"
)

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if err := Templates.ExecuteTemplate(w, "index.html", s.Prefix); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error rendering template"))
	}
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	w.WriteHeader(http.StatusServiceUnavailable)
}
