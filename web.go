package godm

import (
	"log"
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

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}
