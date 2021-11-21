package godm

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if err := Templates.Execute(w, s.Prefix); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error rendering template"))
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusInternalServerError, "Error rendering template")
		return
	}
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	fname := r.URL.Query().Get("id")
	if fname == "" {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("File cannot be empty"))
		return
	}

	b, err := ioutil.ReadFile("odms/" + fname + ".log")
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusInternalServerError, err)
		return
	}

	w.Write(b)
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
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, err)
		return
	}

	// Start to download the file
	if header.Size > 9999 {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte("file size exceeds limits"))
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, "file size exceeds limits", header.Filename, header.Size)
		return
	}

	if !strings.HasSuffix(header.Filename, ".odm") {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte("file type not accepted"))
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, "file type not accepted", header.Filename)
		return
	}

	// Split the filepath just to be safe
	_, fname := filepath.Split(header.Filename)
	outfile := filepath.Join("odms", fname)
	f, err := os.Create(outfile)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, err)
		return
	}
	defer f.Close()
	// Write to the outfile, the in memory buffer, and the hasher all at once
	b := new(bytes.Buffer)
	if _, err := io.Copy(io.MultiWriter(f, b), file); err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, err)
		return
	}
	odm := &OverDriveMedia{}
	err = xml.Unmarshal([]byte(b.Bytes()), &odm)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, "invalid ODM file:", err)
		return
	}
	if odm.Id == "" || odm.License.AcquisitionUrl == "" {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, "invalid ODM file")
		return
	}

	odm.filename = outfile
	lf, err := os.Create(odm.filename + ".log")
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write([]byte(err.Error()))
		log.Println(r.RemoteAddr, r.RequestURI, http.StatusNotAcceptable, err)
		return
	}
	lf.Close()
	go s.DownloadForWeb(odm)

	http.Redirect(w, r, s.Prefix+"/status?id="+fname, http.StatusTemporaryRedirect)
}

/* Download the ODM file, logging output and threading the file */
func (s *Server) DownloadForWeb(o *OverDriveMedia) {
	// Logfile

	logChan := make(chan string)
	wg2 := &sync.WaitGroup{}
	go func(wg *sync.WaitGroup, logs chan string, filename string) {
		defer wg.Done()
		logf, err := os.OpenFile(filename+".log", os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			fmt.Println("Error opening logfile:", err)
			return
		}
		defer logf.Close()
		for l := range logs {
			fmt.Fprintf(logf, "%+v %s\n", time.Now(), l)
			fmt.Printf("%+v %s\n", time.Now(), l)
		}
	}(wg2, logChan, o.filename+".log")
	wg2.Add(1)

	md, _ := o.GetMetadata()
	logChan <- fmt.Sprintf("LOG: Starting download for %s", md.Title)
	outdir := filepath.Join(s.Outdir, md.GetFolderName())
	if err := os.MkdirAll(outdir, 0755); err != nil {
		logChan <- fmt.Sprintf("ERR: Could not make directory: %s", err)
		close(logChan)
		wg2.Wait()
		return
	}

	format := o.chooseBestFormat()
	url := o.getDownloadUrl(format)
	if url == "" {
		logChan <- "ERR: could not get download url"
		close(logChan)
		wg2.Wait()
		return
	}
	if _, err := o.GetLicense(); err != nil {
		logChan <- fmt.Sprintf("ERR: Could not get license: %s", err)
		close(logChan)
		wg2.Wait()
		return
	}
	logChan <- "LOG: Downloaded license file"

	type d struct {
		outfile string
		p       Part
	}

	dataChan := make(chan d)

	wg := &sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		go func(wg *sync.WaitGroup, dataChan chan d, logChan chan string) {
			defer wg.Done()
			for data := range dataChan {
				if err := o.DownloadPart(data.p, data.outfile); err != nil {
					logChan <- fmt.Sprintf("ERR: Could not download part %s: %s", data.p.Number, err)
					continue
				}
				logChan <- fmt.Sprintf("LOG: Saving part %s", data.p.Number)
			}
		}(wg, dataChan, logChan)
		wg.Add(1)
	}

	for _, part := range o.chooseBestFormat().Parts.Part {
		filenameParts := strings.Split(part.FileName, "-")
		filename := filenameParts[len(filenameParts)-1]
		filename = filepath.Join(outdir, filename)
		if s, err := os.Stat(filename); err == nil {
			if s.Size() == int64(part.FileSize) {
				logChan <- fmt.Sprintf("LOG: Part %s already downloaded, skipping", part.Number)
				continue
			}
		}
		dataChan <- d{
			outfile: filename,
			p:       part,
		}
	}

	close(dataChan)
	wg.Wait()
	close(logChan)
	wg2.Wait()
}

func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}
