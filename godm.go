package godm

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

//go:embed static/*
var Files embed.FS

//go:embed index.html
var index string
var Templates = template.Must(template.New("").Parse(index))

type App struct {
	Download Download `cmd help:"Download the ODM file contents"`
	Return   Return   `cmd help:"Return the ODM file"`
	Server   Server   `cmd help:"Serve a website to automatically download books"`
}

type Download struct {
	Odm     string `arg help:"ODM File to parse"`
	Outdir  string `arg help:"out directory to save files to"`
	Return  bool   `short:"r" help:"return the book when successfully downloaded"`
	Verbose bool   `short:"v" help:"Print more information"`
}

func (d *Download) Run() error {
	fmt.Println("Parsing ODM file")
	odm, err := NewODMFile(d.Odm)
	if err != nil {
		return err
	}
	fmt.Println("Acquiring License")
	if _, err := odm.GetLicense(); err != nil {
		return err
	}
	fmt.Println("Downloading all parts")
	if err = odm.Download(d.Outdir, 10, d.Verbose); err != nil {
		return err
	}
	// TODO Validate the download worked
	if d.Return {
		fmt.Println("Returning book")
		return odm.Return()
	}
	return nil
}

type Return struct {
	Odm string `arg help:"ODM File to return"`
}

func (r *Return) Run() error {
	fmt.Println("Parsing ODM file")
	odm, err := NewODMFile(r.Odm)
	if err != nil {
		return err
	}
	fmt.Println("Returning book")
	return odm.Return()
}

type Server struct {
	Address string `short:"a" help:"Address to listen on (env GODM_ADDR)" default:":8080"`
	Prefix  string `short:"p" help:"URL prefix to use (env GODM_PREFIX)"`
	Outdir  string `arg help:"out directory to save files to (env GODM_OUTDIR)"`
	Verbose bool   `short:"v" help:"Print more information"`
}

func (s *Server) Run() error {
	if pr := os.Getenv("GODM_PREFIX"); pr != "" {
		s.Prefix = pr
	}
	if ad := os.Getenv("GODM_ADDR"); ad != "" {
		s.Address = ad
	}
	if outdir := os.Getenv("GODM_OUTDIR"); outdir != "" {
		s.Outdir = outdir
	}

	if len(s.Prefix) != 0 && s.Prefix[0] != '/' {
		return fmt.Errorf("URL prefix does not begin with '/'")
	}

	if err := os.Mkdir("odms", 0755); err != nil {
		return fmt.Errorf("cannot make tempory output directory: %s", err)
	}

	routes := http.NewServeMux()
	routes.Handle("/static/", http.FileServer(http.FS(Files)))
	routes.HandleFunc("/", s.index)
	routes.HandleFunc("/upload", s.upload)
	routes.HandleFunc("/status", s.status)
	lggr := logRequest(routes)
	log.Println("Serving HTTP on", s.Address, "with prefix", s.Prefix, "saving to", s.Outdir)
	if len(s.Prefix) != 0 {
		prefix := http.StripPrefix(s.Prefix, lggr)
		return http.ListenAndServe(s.Address, prefix)
	} else {
		return http.ListenAndServe(s.Address, lggr)
	}
}

type data struct {
	r *http.Request
	f string
}

/* Worker that downloads requests to a file */
func worker(wg *sync.WaitGroup, c chan data, e chan error, verbose bool) {
	defer wg.Done()
	client := http.Client{}
	for d := range c {
		f, err := os.Create(d.f)
		if err != nil {
			e <- err
			continue
		}

		resp, err := client.Do(d.r)
		if err != nil {
			e <- err
			continue
		}

		_, err = io.Copy(f, resp.Body)
		if err != nil {
			e <- err
			continue
		}

		if verbose {
			log.Println("Saved file %s", d.f)
		}
	}
}
