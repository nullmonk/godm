package godm

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mikkyang/id3-go"
)

type Marker struct {
	Name    string
	Time    string
	EndTime string
}

func (m *Marker) String() string {
	return fmt.Sprintf("%s: %s-%s", m.Name, m.Time, m.EndTime)
}

func (m *Marker) NormalizeName() string {
	r := strings.ReplaceAll(m.Name, ".", "")
	r = strings.ReplaceAll(r, "\"", "")
	return strings.ReplaceAll(r, "'", "")
}

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

type Audiobook struct {
	// filename: Markers
	Title       string
	Description string
	Markers     map[string][]*Marker
	M3U         []string // The M3U file
}

func NewAudiobook() *Audiobook {
	return &Audiobook{
		Markers: make(map[string][]*Marker),
		M3U:     make([]string, 0),
	}
}

/* Check if a chapter is conitnued, or if it has the same name as another chapter
If it does have the same name, add II
*/
func (a *Audiobook) IsContinued(marker *Marker) bool {
	for j, title := range a.M3U {
		if title == marker.Name {
			// Chapter is carrying over from the last segment
			if j == len(a.M3U)-1 {
				return true
			}
			if strings.HasSuffix(title, "I") {
				marker.Name = title + "I"
			} else {
				marker.Name += " II"
			}
		}
	}
	return false
}

func (a *Audiobook) Add(filename string, markers []*Marker) error {
	if _, ok := a.Markers[filename]; ok {
		return fmt.Errorf("file already added")
	}

	a.Markers[filename] = markers
	for i, m := range markers {
		if err := m.NormalizeTime(); err != nil {
			return err
		}
		if i != 0 {
			// Add the end time to the previous marker
			markers[i-1].EndTime = m.Time
		}
		// Add the chapter to the m3u if it hasnt been repeated
		if !a.IsContinued(m) {
			a.M3U = append(a.M3U, m.Name)
		}
	}
	return nil
}

func (a *Audiobook) FindChapterIndex(name string) int {
	for i, marker := range a.M3U {
		if marker == name {
			return i
		}
	}
	return -1
}

type ParseChapters struct {
	Directory string `arg help:"directory to parse"`
	Outdir    string `arg help:"out directory to save files to" optional:""`
	Verbose   bool   `short:"v" help:"Print more information"`
	Delete    bool   `short:"d" help:"Delete previous parts on success"`

	book *Audiobook
}

func (p *ParseChapters) Run() error {
	if p.Outdir == "" {
		p.Outdir = p.Directory
	} else {
		os.MkdirAll(p.Outdir, 0755)
	}
	p.book = NewAudiobook()
	var doesDescriptionExist bool
	var description string
	//doesAlbumCoverExists := false
	err := filepath.Walk(p.Directory, func(path string, info fs.FileInfo, err error) error {
		switch filepath.Ext(info.Name()) {
		case ".txt":
			if info.Size() > 0 {
				doesDescriptionExist = true
			}
			return nil
		case ".mp3":
			f, err := id3.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			// Get the overdirve media tags
			media := strings.SplitN(f.Frame("TXXX").String(), ":", 2)
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

			for _, frame := range f.AllFrames() {
				fmt.Println(frame.Id(), frame.String())
			}

			if p.book.Title == "" {
				p.book.Title = f.Frame("TIT3").String()
			}
			p.book.Add(path, markers.Markers)
			if description == "" {
				desc := strings.SplitN(f.Frame("COMM").String(), ":", 2)
				if len(desc) > 1 {
					description = fmt.Sprintf("%s %s\n\n%s\n\n%s",
						f.Frame("TIT3").String(),
						f.Frame("TPE1").String(),
						strings.TrimSpace(desc[1]),
						f.Frame("TCON").String())
				}
			}
		default:
			return nil
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !doesDescriptionExist && description != "" {
		if err := ioutil.WriteFile(filepath.Join(p.Outdir, "Description.txt"), []byte(description), 0644); err != nil {
			return err
		}
	}

	var wasError error
	for filename, markers := range p.book.Markers {
		for _, marker := range markers {
			destPrefix := filepath.Join(p.Outdir, fmt.Sprintf("%02d - %s", p.book.FindChapterIndex(marker.Name), marker.NormalizeName()))
			destination := destPrefix + ".mp3"
			i := 0
			for {
				if _, err := os.Stat(destination); err == nil {
					i++
					destination = fmt.Sprintf("%s_%d.mp3", destPrefix, i)
				}
				break
			}
			if err := SplitMP3(filename, destination, marker); err != nil {
				wasError = err
				fmt.Println(err)
			}
		}
		if wasError == nil && p.Delete {
			os.Remove(filename)
		}
		wasError = nil
	}
	return nil
}

func GetIndex(arr []string, name string) int {
	for i, val := range arr {
		if val == name {
			return i
		}
	}
	return -1
}
