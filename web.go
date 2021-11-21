package godm

import (
	"fmt"
	"log"
	"net/http"
)

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if err := Templates.Execute(w, s.Prefix); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error rendering template"))
	}
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	_, header, err := r.FormFile("odmFile")
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
	}

	log := fmt.Sprintf("Got file: %s %s", header.Filename, header.Header)
	w.Write([]byte(log))
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}
