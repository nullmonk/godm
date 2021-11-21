package godm

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
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

	"github.com/google/uuid"
	"golang.org/x/text/encoding/unicode"
)

const (
	OMC       = "1.2.0"
	OS        = "10.11.6"
	UserAgent = "OverDrive Media Console" // same user agent as mobile app
)

type LicenseFile struct {
	XMLName    xml.Name
	SignedInfo struct {
		ClientID string
	}
	ErrorMessage string
}

type Metadata struct {
	ContentType  string
	Title        string
	CoverUrl     string
	ThumbnailUrl string
	Creators     struct {
		Creators []struct {
			Role string `xml:"role,attr"`
			Name string `xml:",innerxml"`
		} `xml:"Creator"`
	}
}

func (m Metadata) GetAuthor() string {
	if len(m.Creators.Creators) == 0 {
		return "Author Unknown"
	}
	for _, c := range m.Creators.Creators {
		if strings.ToLower(c.Role) == "author" {
			return c.Name
		}
	}
	return m.Creators.Creators[0].Name
}

func (m Metadata) GetFolderName() string {
	return strings.ReplaceAll(fmt.Sprintf("%s_%s", m.GetAuthor(), m.Title), " ", "")
}

type Part struct {
	Number   string `xml:"number,attr"`
	FileSize int    `xml:"filesize,attr"`
	Name     string `xml:"name,attr"`
	FileName string `xml:"filename,attr"`
}

type Parts struct {
	Count int `xml:"count,attr"`
	Part  []Part
}

type Formats struct {
	Formats []Format `xml:"Format"`
}

type Format struct {
	Name    string `xml:"name,attr"`
	Quality struct {
		Level string `xml:"level,attr"`
	}
	Protocols struct {
		Protocol []struct {
			Method string `xml:"method,attr"`
			Url    string `xml:"baseurl,attr"`
		}
	}
	Parts Parts
}

type OverDriveMedia struct {
	ClientID string
	Id       string `xml:"id,attr"`
	License  struct {
		AcquisitionUrl string
		License        string
	}
	DrmInfo struct {
		ExpirationDate string
	}
	Formats        Formats
	Metadata       string `xml:",chardata"`
	EarlyReturnURL string
	TransactionID  string

	data     []byte
	filename string
}

func NewODMFile(filename string) (*OverDriveMedia, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	odm := &OverDriveMedia{}
	err = xml.Unmarshal([]byte(data), &odm)
	if err != nil {
		return nil, fmt.Errorf("invalid ODM file: %s", err)
	}
	if odm.Id == "" || odm.License.AcquisitionUrl == "" {
		return nil, fmt.Errorf("invalid ODM file")
	}
	odm.data = data
	odm.filename = filename
	return odm, nil
}

func (o *OverDriveMedia) GetMetadata() (*Metadata, error) {
	meta := &Metadata{}
	err := xml.Unmarshal([]byte(o.Metadata), &meta)
	return meta, err
}

func (o *OverDriveMedia) GenHash() string {
	o.ClientID = strings.ToUpper(uuid.New().String())
	/* Special thanks to https://github.com/chbrown/overdrive/ and https://github.com/jvolkening/gloc
	for figuring out the hash function */
	hashValue := fmt.Sprintf("%s|%s|%s|ELOSNOC*AIDEM*EVIRDREVO", o.ClientID, OMC, OS)
	enc := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	bytes, _ := enc.Bytes([]byte(hashValue))
	data := sha1.Sum(bytes)
	return base64.StdEncoding.EncodeToString(data[:])
}

/* Get the license from the server. Ifa license has already been checked out, then it will
error so we should be careful to cache the license file */
func (o *OverDriveMedia) GetLicense() (string, error) {
	outfile := o.filename + ".license"
	if len(o.License.License) != 0 {
		return o.License.License, nil
	}
	if data, err := ioutil.ReadFile(outfile); err == nil {
		o.License.License = string(data)
		lf := &LicenseFile{}
		err = xml.Unmarshal(data, &lf)
		if err != nil {
			return "", err
		}
		o.ClientID = lf.SignedInfo.ClientID
		fmt.Println("Using cache")
		return string(data), nil
	}
	client := &http.Client{}
	hash := o.GenHash()
	url := fmt.Sprintf("%s?MediaID=%s&ClientID=%s&OMC=%s&OS=%s&Hash=%s",
		o.License.AcquisitionUrl, o.Id, o.ClientID, OMC, OS, hash)

	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	r.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(r)
	if err != nil {
		return "", err
	}
	b := new(bytes.Buffer)
	io.Copy(b, resp.Body)
	// Validate that there isnt an error in the license
	lf := &LicenseFile{}
	err = xml.Unmarshal(b.Bytes(), &lf)
	if err != nil {
		return "", err
	}
	if lf.ErrorMessage != "" {
		return "", fmt.Errorf(lf.ErrorMessage)
	}

	// No error, save license
	outf, err := os.Create(outfile)
	if err != nil {
		return "", err
	}
	o.License.License = b.String()
	outf.Write(b.Bytes())
	return b.String(), err
}

func (o *OverDriveMedia) getDownloadUrl(format Format) string {
	for _, p := range format.Protocols.Protocol {
		if strings.ToLower(p.Method) == "download" {
			return p.Url
		}
	}
	return ""
}

/* Some files might have more than one format, if so, choose "High" */
func (o *OverDriveMedia) chooseBestFormat() Format {
	if len(o.Formats.Formats) == 1 {
		return o.Formats.Formats[0]
	}

	quality := map[string]int{
		"Low":    0,
		"Medium": 1,
		"High":   2,
	}
	var highest *Format
	for _, format := range o.Formats.Formats {
		if highest == nil {
			highest = &format
			continue
		}
		if quality[format.Quality.Level] > quality[highest.Quality.Level] {
			highest = &format
			continue
		}

	}
	return *highest
}

func (o *OverDriveMedia) Return() error {
	_, err := http.Get(o.EarlyReturnURL)
	return err
}

func (o *OverDriveMedia) DownloadPart(p Part, outfile string) error {
	outf, err := os.Create(outfile)
	if err != nil {
		return err
	}
	license, err := o.GetLicense()
	if err != nil {
		return fmt.Errorf("could not get license")
	}
	format := o.chooseBestFormat()
	url := o.getDownloadUrl(format)
	if url == "" {
		return fmt.Errorf("could not get download url")
	}
	r, err := http.NewRequest("GET", url+"/"+p.FileName, nil)
	if err != nil {
		return err
	}
	r.Header.Set("User-Agent", UserAgent)
	r.Header.Set("ClientID", o.ClientID)
	r.Header.Set("License", license)
	client := http.Client{}
	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("invalid status code received: %d", resp.StatusCode)
	}
	_, err = io.Copy(outf, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

/* Download all the parts */
func (o *OverDriveMedia) Download(outdir string, threads int, verbose bool) error {
	// Make sure we have the license
	license, err := o.GetLicense()
	if err != nil {
		return err
	}
	md, _ := o.GetMetadata()
	outdir = filepath.Join(outdir, md.GetFolderName())
	if err := os.MkdirAll(outdir, 0755); err != nil {
		return err
	}

	format := o.chooseBestFormat()
	url := o.getDownloadUrl(format)
	if url == "" {
		return fmt.Errorf("could not get download url")
	}

	dataChan := make(chan data)
	errChan := make(chan error)
	wg := &sync.WaitGroup{}
	for i := 0; i < threads; i++ {
		go worker(wg, dataChan, errChan, verbose)
		wg.Add(1)
	}
	f, err := os.Create(filepath.Join(outdir, o.filename))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	f.Write(o.data)

	for _, part := range format.Parts.Part {
		r, err := http.NewRequest("GET", url+"/"+part.FileName, nil)
		if err != nil {
			return err
		}
		r.Header.Set("User-Agent", UserAgent)
		r.Header.Set("ClientID", o.ClientID)
		r.Header.Set("License", license)
		filenameParts := strings.Split(part.FileName, "-")
		filename := filenameParts[len(filenameParts)-1]
		filename = filepath.Join(outdir, filename)
		if s, err := os.Stat(filename); err == nil {
			if s.Size() == int64(part.FileSize) {
				continue
			}
		}

		dataChan <- data{
			f: filename,
			r: r,
		}
	}

	albumArt := filepath.Join(outdir, "folder.jpg")
	if i, err := os.Stat(albumArt); err == nil && i.Size() != 0 {
		// Already have it
	} else {
		r, err := http.NewRequest("GET", md.CoverUrl, nil)
		if err != nil {
			return err
		}
		dataChan <- data{
			f: albumArt,
			r: r,
		}
	}

	close(dataChan)
	wg.Wait()
	return nil
}
