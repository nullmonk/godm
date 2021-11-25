package godm

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"strings"
)

func SplitMP3(filename, destination string, marker *Marker) error {

	comm := []string{
		"-i",
		"" + filename + "",
		"-acodec",
		"copy",
		"-ss",
		marker.Time,
	}

	if marker.EndTime != "" {
		comm = append(comm, "-to", marker.EndTime)
	}
	comm = append(comm, destination)
	com := exec.Command("ffmpeg", comm...)
	stderr, _ := com.StderrPipe()
	if err := com.Run(); err != nil {
		fmt.Println("ffmpeg", strings.Join(comm, " "))
		o, _ := ioutil.ReadAll(stderr)
		return fmt.Errorf("%s %s", err, o)
	}
	return nil
}
