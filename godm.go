package godm

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
)

type App struct {
	Download Download `cmd help:"Download the ODM file contents"`
	Return   Return   `cmd help:"Return the ODM file"`
	Server   Serve    `cmd help:"Serve a website to automatically download books"`
}

type Download struct {
	Odm    string `arg help:"ODM File to parse"`
	Outdir string `arg help:"out directory to save files to"`
	Return bool   `short:"r" help:"return the book when successfully downloaded"`
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
	if err = odm.Download(d.Outdir, 10); err != nil {
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

type Serve struct {
	Address string `short:"a" help:"Address to listen on" default:":8080"`
	Outdir  string `arg help:"out directory to save files to"`
}

func (s *Serve) Run() error {
	staticAssets := http.FileServer(http.Dir("static/"))
	http.Handle("/static/", http.StripPrefix("/static/", staticAssets))
	http.HandleFunc("/", index)
	http.HandleFunc("/upload", upload)
	log.Println("Serving HTTP on", s.Address, "saving to", s.Outdir)
	http.ListenAndServe(s.Address, nil)
	return nil
}

type data struct {
	r *http.Request
	f string
}

/* Worker that downloads requests to a file */
func worker(wg *sync.WaitGroup, c chan data, e chan error) {
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
		client.Do(d.r)
	}
}
