package godm

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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
		return
	}
	file, header, err := r.FormFile("odmFile")
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
		log.Print(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, err)
		return
	}

	// Start to download the file
	if header.Size > 9999 {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte("file size exceeds limits"))
		log.Print(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, "file size exceeds limits", header.Filename, header.Size)
		return
	}

	if !strings.HasSuffix(header.Filename, ".odm") {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte("file type not accepted"))
		log.Print(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, "file type not accepted", header.Filename)
		return
	}

	// Split the filepath just to be safe
	_, fname := filepath.Split(header.Filename)
	outfile := filepath.Join("odms", fname)
	f, err := os.Create(outfile)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
		log.Print(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, err)
		return
	}
	defer f.Close()
	// Write to the outfile, the in memory buffer, and the hasher all at once
	hasher := sha1.New()
	b := new(bytes.Buffer)
	if _, err := io.Copy(io.MultiWriter(f, b, hasher), file); err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
		log.Print(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, err)
		return
	}

	hash := hex.EncodeToString(hasher.Sum(nil))
	os.Rename(outfile, hash+".odm")
	http.Redirect(w, r, s.Prefix+"/status?id="+hash, http.StatusTemporaryRedirect)
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}
