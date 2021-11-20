package main

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

	"github.com/google/uuid"
	"golang.org/x/text/encoding/unicode"
)

const (
	OMC       = "1.2.0"
	OS        = "10.11.6"
	UserAgent = "OverDrive Media Console" // same user agent as mobile app
)

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
	FileSize string `xml:"filesize,attr"`
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

func (o *OverDriveMedia) GetLicense(outfile string) (string, error) {
	if data, err := ioutil.ReadFile(outfile); err == nil {
		o.License.License = string(data)
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
	outf, err := os.Create(outfile)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(r)
	if err != nil {
		return "", err
	}
	b := new(bytes.Buffer)
	output := io.MultiWriter(b, outf)
	io.Copy(output, resp.Body)
	o.License.License = b.String()
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

func (o *OverDriveMedia) chooseBestFormat() Format {
	if len(o.Formats.Formats) == 1 {
		return o.Formats.Formats[0]
	}

	quality := map[string]int{
		"Low":    1,
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

/* Download all the parts */
func (o *OverDriveMedia) Download(outdir string) error {
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

	for _, part := range format.Parts.Part {
		r, err := http.NewRequest("GET", url+"/"+part.FileName, nil)
		if err != nil {
			return err
		}
		r.Header.Set("User-Agent", UserAgent)
		r.Header.Set("ClientID", o.ClientID)
		r.Header.Set("License", o.License.License)
		filenameParts := strings.Split(part.FileName, "-")
		filename := filenameParts[len(filenameParts)-1]
		fmt.Println(filepath.Join(outdir, filename))
		fmt.Println(url, part.FileName)
	}
	return nil

}
func (o *OverDriveMedia) Dump() {
	//fmt.Println("Title:", o.Metadata.Title)
	fmt.Println("Media ID:", o.Id)
	fmt.Println("Content Type:", o.Metadata)
	fmt.Println("Acquisition URL:", o.License.AcquisitionUrl)
}

func main() {
	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	odm := &OverDriveMedia{}
	err = xml.Unmarshal([]byte(data), &odm)
	if err != nil {
		log.Fatal(err)
	}

	odm.GetLicense(os.Args[1] + ".license")
	if err := odm.Download("extract"); err != nil {
		log.Fatal(err)
	}
}
