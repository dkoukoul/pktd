package main

import (
	"log"

	gendoc "github.com/pseudomuto/protoc-gen-doc"
	"github.com/pseudomuto/protokit"
)

func main() {
	if err := protokit.RunPlugin(new(gendoc.Plugin)); err != nil {
		log.Fatal(err)
	}
}
