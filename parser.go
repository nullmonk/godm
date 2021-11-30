package godm

import (
	"archive/zip"
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mikkyang/id3-go"
	v2 "github.com/mikkyang/id3-go/v2"
)

// Remove timestamps from the title as overdrive does this sometimes. e.g. Chapter 7 (00:00)
var TITLE_RE = regexp.MustCompile(`(\s+\(([0-9]+:)+[0-9]+\))$`)

// Remove "Part N - " from the title
//var TITLE_RE_2 = regexp.MustCompile(`(\s*(Part)\s+\d(\s+\-\s+))`)

type Marker struct {
	Name    string
	Time    string
	EndTime string
	Source  string // Source filename
}

func (m *Marker) String() string {
	return fmt.Sprintf("%s: %s-%s", m.Name, m.Time, m.EndTime)
}

func (m *Marker) NormalizeName() string {
	m.Name = TITLE_RE.ReplaceAllString(m.Name, "")
	//m.Name = TITLE_RE_2.ReplaceAllString(m.Name, "")
	m.Name = strings.TrimSpace(m.Name)
	m.Name = strings.ReplaceAll(m.Name, ".", "")
	m.Name = strings.ReplaceAll(m.Name, "\"", "")
	m.Name = strings.ReplaceAll(m.Name, "/", "-")
	m.Name = strings.ReplaceAll(m.Name, "|", "-")
	m.Name = strings.ReplaceAll(m.Name, "'", "")
	m.Name = strings.ReplaceAll(m.Name, "?", "")
	return m.Name
}

/*
Normalize time markers for FFMPEG. Overdrive uses minutes > 60, splt these into hours
*/
func (m *Marker) NormalizeTime() (err error) {
	var h, min int
	var s float64

	times := strings.Split(m.Time, ":")
	switch len(times) {
	case 0:
		return fmt.Errorf("empty time block")
	case 3:
		h, err = strconv.Atoi(times[len(times)-3])
		if err != nil {
			return err
		}
		fallthrough
	case 2:
		min, err = strconv.Atoi(times[len(times)-2])
		if err != nil {
			return err
		}
		fallthrough
	case 1:
		s, err = strconv.ParseFloat(times[len(times)-1], 64)
		if err != nil {
			return err
		}
	}

	h += min / 60
	min = min % 60

	change := int(math.Floor(s / 60))
	min += change
	s -= float64(60 * change)

	m.Time = fmt.Sprintf("%02d:%02d:%0.3f", h, min, s)
	return
}

type Markers struct {
	Markers []*Marker `xml:"Marker"`
}

type ParseChapters struct {
	Directory string `arg help:"directory to parse"`
	Outdir    string `arg help:"out directory to save files to" optional:""`
	Delete    bool   `short:"d" help:"Delete previous parts on success"`

	logfile io.Writer

	allMarkers []*Marker
}

func (p *ParseChapters) parsePlaylist(filename string) ([]string, error) {
	var files []string
	fil, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	scanner := bufio.NewScanner(fil)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 || line[0] == '#' || strings.Contains(line, "EXTM3U") {
			continue
		}

		files = append(files, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return files, nil
}

func (p *ParseChapters) Run() error {
	if p.logfile == nil {
		p.logfile = os.Stdout
	}
	if p.Outdir == "" {
		p.Outdir = p.Directory
	} else {
		os.MkdirAll(p.Outdir, 0755)
	}

	p.allMarkers = make([]*Marker, 0)
	// Generate a description for the book if needed
	var author, categories, summary, description string

	sourceFiles := make([]string, 0)

	var playlist []string

	err := filepath.Walk(p.Directory, func(path string, info fs.FileInfo, err error) error {
		switch filepath.Ext(info.Name()) {
		case ".m3u":
			playlist, err = p.parsePlaylist(path)
			return err
		case ".txt":
			fallthrough
		case ".html":
			if info.Size() > 0 {
				description = "default"
			}
			return nil
		case ".mp3":
			f, err := id3.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			// Get the overdrive media tags
			var v v2.Framer
			if v = f.Frame("TXXX"); v == nil {
				return nil
			}
			media := strings.SplitN(v.String(), ":", 2)
			if len(media) < 1 {
				return nil
			}
			if !strings.Contains(media[0], "OverDrive MediaMarkers") {
				return nil
			}
			markers := &Markers{}
			if err := xml.Unmarshal([]byte(media[1]), markers); err != nil {
				return err
			}

			// set the title, author, for the description
			if v := f.Frame("TPE1"); author == "" && v != nil && v.String() != "" {
				author = v.String()
			}
			if v := f.Frame("TCON"); categories == "" && v != nil && v.String() != "" {
				categories = v.String()
			}
			if v := f.Frame("COMM"); summary == "" && v != nil && v.String() != "" {
				summary = strings.TrimSpace(strings.SplitN(v.String(), ":", 2)[1])
			}

			// Normalize the markers
			for i, m := range markers.Markers {
				m.NormalizeName()
				m.Source = path
				if err := m.NormalizeTime(); err != nil {
					fmt.Fprintf(p.logfile, "%+v ERR: Cannot normalize time for file %s: %s\n", time.Now(), path, err)
				}
				if i != 0 {
					// Add the end time to the previous marker
					prev := p.allMarkers[len(p.allMarkers)-1]
					if prev.Source == m.Source {
						prev.EndTime = m.Time
					}
				}
				p.allMarkers = append(p.allMarkers, m)
			}

			sourceFiles = append(sourceFiles, path)

		default:
			return nil
		}
		return nil
	})

	if err != nil {
		fmt.Fprintf(p.logfile, "%+v ERR: Walking directory: %s\n", time.Now(), err)
		return err
	}

	if p.allMarkers != nil && len(p.allMarkers) == 0 && playlist != nil {
		fmt.Fprintf(p.logfile, "%+v Detected Playlist File with no Overdrive Markers.\n", time.Now())
		formatStr := fmt.Sprintf("%%0%dd - %%s.mp3", len(fmt.Sprint(len(p.allMarkers))))

		for i, fname := range playlist {
			destination := filepath.Join(p.Outdir, fmt.Sprintf(formatStr, i, fname))
			os.Rename(filepath.Join(p.Directory, fname), destination)
			fmt.Fprintf(p.logfile, "%+v Renamed %s\n", time.Now(), fname)
		}
		fmt.Fprintf(p.logfile, "%+v %s\n", time.Now(), "Renamed files")
		return nil
	}
	// Write the description if we dont have one
	if description == "" && summary != "" {
		description = fmt.Sprintf("%s<br><br>\n%s\n<br>\n%s", summary, author, categories)
		if err := ioutil.WriteFile(filepath.Join(p.Outdir, "about.html"), []byte(description), 0644); err != nil {
			return err
		}
		fmt.Fprintf(p.logfile, "%+v %s\n", time.Now(), "Saved description to about.txt")

	}

	// Print out all the markers
	i := 0
	// Format string used for output files, zeros padded as much as needed
	formatStr := fmt.Sprintf("%%0%dd - %%s.mp3", len(fmt.Sprint(len(p.allMarkers))))
	for _, marker := range p.allMarkers {
		destination := filepath.Join(p.Outdir, fmt.Sprintf(formatStr, i, marker.Name))
		if err := SplitMP3(marker.Source, destination, marker); err != nil {
			fmt.Fprintf(p.logfile, "%+v ERR: Could not split file: %s\n", time.Now(), err)
		}
		fmt.Fprintf(p.logfile, "%+v Saved %d - %s\n", time.Now(), i, marker.Name)
		i++
	}

	// Package the old Parts into a zipfile
	if !p.Delete {
		return nil
	}
	_, file := filepath.Split(strings.TrimRight(p.Directory, "/"))
	file = filepath.Join(p.Outdir, "..", file+".zip")
	of, err := os.Create(file) // Output zipfile
	if err != nil {
		fmt.Fprintf(p.logfile, "%+v ERR: Could not create output zipfile: %s: %s\n", time.Now(), file, err)
		return err
	}
	defer of.Close()
	zf := zip.NewWriter(of) // Output zip writer
	defer zf.Close()
	for _, sourceFile := range sourceFiles {
		_, fname := filepath.Split(sourceFile)
		f, err := zf.Create(fname)
		if err != nil {
			fmt.Fprintf(p.logfile, "%+v ERR: Could not create file in zipfile: %s: %s\n", time.Now(), fname, err)
			return err
		}
		sf, err := os.Open(sourceFile)
		if err != nil {
			fmt.Fprintf(p.logfile, "%+v ERR: Could not read file: %s: %s\n", time.Now(), sourceFile, err)
			return err
		}
		defer sf.Close()
		if _, err := io.Copy(f, sf); err != nil {
			fmt.Fprintf(p.logfile, "%+v ERR: Could not read file: %s: %s\n", time.Now(), sourceFile, err)
			return err
		}
		if err := os.Remove(sourceFile); err != nil {
			fmt.Fprintf(p.logfile, "%+v ERR: Could not delete file: %s: %s\n", time.Now(), sourceFile, err)
			return err
		}
	}
	fmt.Fprintf(p.logfile, "%+v Saved original files to %s\n", time.Now(), file)
	return nil
}
