package main

import (
	"godm"
	"log"

	"github.com/alecthomas/kong"
)

func main() {
	app := &godm.App{}
	ctx := kong.Parse(app)
	if err := ctx.Run(); err != nil {
		log.Fatal(err)
	}
}
