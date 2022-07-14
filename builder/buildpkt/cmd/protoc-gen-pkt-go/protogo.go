// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import (
	"flag"
	"fmt"
	"strings"
	_ "unsafe"

	gengo "google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
)

// This file is based on the protoc-gen-go code
// https://github.com/golang/protobuf/blob/v1.5.2/protoc-gen-go/main.go
// It contains just enough code to invoke protoc-gen-go internals as a library.
// It is done this way in order that we don't need the person compiling pktd to install
// protoc-gen-go on their computer.

func main() {
	var (
		flags        flag.FlagSet
		plugins      = flags.String("plugins", "", "list of plugins to enable (supported values: grpc)")
		importPrefix = flags.String("import_prefix", "", "prefix to prepend to import paths")
	)
	importRewriteFunc := func(importPath protogen.GoImportPath) protogen.GoImportPath {
		switch importPath {
		case "context", "fmt", "math":
			return importPath
		}
		if *importPrefix != "" {
			return protogen.GoImportPath(*importPrefix) + importPath
		}
		return importPath
	}
	protogen.Options{
		ParamFunc:         flags.Set,
		ImportRewriteFunc: importRewriteFunc,
	}.Run(func(gen *protogen.Plugin) error {
		for _, plugin := range strings.Split(*plugins, ",") {
			if plugin != "" {
				return fmt.Errorf("protoc-gen-go: unknown plugin %q", plugin)
			}
		}
		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}
			gengo.GenerateFile(gen, f)
		}
		gen.SupportedFeatures = gengo.SupportedFeatures
		return nil
	})
}
