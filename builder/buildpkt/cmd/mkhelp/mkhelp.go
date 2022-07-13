package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	gendoc "github.com/pseudomuto/protoc-gen-doc"
)

const prologue string = `
package pkthelp

type Field struct {
	Name        string
	Description []string
	Repeated    bool
	Type        Type
}

type Varient struct {
	Name        string
	Description []string
}

type Type struct {
	Name        string
	Description []string
	Fields      []Field
}

type Method struct {
	Name             string
	Service          string
	Category         string
	ShortDescription string
	Description      []string
	Req              Type
	Res              Type
}

var EnumVarientType Type = Type{
	Name: "ENUM_VARIENT",
}

`

func desc(desc string, padding string) {
	if len(desc) > 0 {
		fmt.Printf("%sDescription: []string{\n", padding)
		for _, l := range strings.Split(desc, "\n") {
			fmt.Printf("%s    %s,\n", padding, strconv.Quote(l))
		}
		fmt.Printf("%s},\n", padding)
	}
}

func fixName(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: genhelp dirpath")
		os.Exit(100)
		return
	}
	path := os.Args[len(os.Args)-1]
	var templates []gendoc.Template
	filepath.Walk(path, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			panic(err)
		}
		if !strings.HasSuffix(file, ".doc.json") {
			return nil
		}
		content, err := os.ReadFile(file)
		if err != nil {
			panic(err.Error())
		}
		t := gendoc.Template{}
		if err := json.Unmarshal(content, &t); err != nil {
			panic(err.Error())
		}
		templates = append(templates, t)
		return nil
	})
	fmt.Print(prologue)
	for _, t := range templates {
		for _, s := range t.Scalars {
			fmt.Printf("func mk%s() Type {\n", s.ProtoType)
			fmt.Printf("    return Type{\n")
			fmt.Printf("        Name: %s,\n", strconv.Quote(s.GoType))
			fmt.Printf("    }\n")
			fmt.Printf("}\n")
		}
		break
	}
	for _, t := range templates {
		for _, f := range t.Files {
			for _, e := range f.Enums {
				fmt.Printf("func mk%s() Type {\n", fixName(e.FullName))
				fmt.Printf("    return Type{\n")
				fmt.Printf("        Name: %s,\n", strconv.Quote(fixName(e.FullName)))
				desc(e.Description, "        ")
				fmt.Printf("        Fields: []Field{\n")
				for _, v := range e.Values {
					fmt.Printf("            {\n")
					fmt.Printf("                Name: %s,\n", strconv.Quote(v.Name))
					desc(v.Description, "                ")
					fmt.Printf("                Type: EnumVarientType,\n")
					fmt.Printf("            },\n")
				}
				fmt.Printf("        },\n")
				fmt.Printf("    }\n")
				fmt.Printf("}\n")
			}
		}
	}
	for _, t := range templates {
		for _, f := range t.Files {
			for _, e := range f.Messages {
				fmt.Printf("func mk%s() Type {\n", fixName(e.FullName))
				fmt.Printf("    return Type{\n")
				fmt.Printf("        Name: %s,\n", strconv.Quote(fixName(e.FullName)))
				desc(e.Description, "        ")
				if len(e.Fields) > 0 {
					fmt.Printf("        Fields: []Field{\n")
					for _, f := range e.Fields {
						fmt.Printf("            {\n")
						fmt.Printf("                Name: %s,\n", strconv.Quote(f.Name))
						desc(f.Description, "                ")
						if f.Label == "repeated" {
							fmt.Printf("                Repeated: true,\n")
						}
						fmt.Printf("                Type: mk%s(),\n", fixName(f.FullType))
						fmt.Printf("            },\n")
					}
					fmt.Printf("        },\n")
				}
				fmt.Printf("    }\n")
				fmt.Printf("}\n")
			}
		}
	}

	var categoryRegexp *regexp.Regexp
	var shortDescriptionRegexp *regexp.Regexp

	categoryRegexp, err := regexp.Compile("\\$pld\\.category:\\s*`([^`]+)`")
	if err != nil {
		panic(err.Error())
	}

	shortDescriptionRegexp, err = regexp.Compile("\\$pld\\.short_description:\\s*`([^`]+)`")
	if err != nil {
		panic(err.Error())
	}

	for _, t := range templates {
		for _, f := range t.Files {
			for _, s := range f.Services {
				for _, m := range s.Methods {
					fmt.Printf("func %s_%s() Method {\n", s.Name, m.Name)
					fmt.Printf("    return Method{\n")
					fmt.Printf("        Name: %s,\n", strconv.Quote(m.Name))
					fmt.Printf("        Service: %s,\n", strconv.Quote(s.Name))
					if len(s.Description) > 0 {

						var match []string
						var matchIndex []int

						match = categoryRegexp.FindStringSubmatch(m.Description)
						if len(match) > 1 {
							fmt.Printf("        Category: %s,\n", strconv.Quote(match[1]))

							matchIndex = categoryRegexp.FindStringIndex(m.Description)
							m.Description = m.Description[0:matchIndex[0]] + m.Description[matchIndex[1]:]
						}

						match = shortDescriptionRegexp.FindStringSubmatch(m.Description)
						if len(match) > 1 {
							fmt.Printf("        ShortDescription: %s,\n", strconv.Quote(match[1]))

							matchIndex = shortDescriptionRegexp.FindStringIndex(m.Description)
							m.Description = m.Description[0:matchIndex[0]] + m.Description[matchIndex[1]:]
						}

						fmt.Printf("        Description: []string{\n")
						for _, s := range strings.Split(m.Description, "\n") {
							descriptionLine := strings.TrimSpace(s)
							if len(descriptionLine) > 0 {
								fmt.Printf("            %s,\n", strconv.Quote(descriptionLine))
							}
						}
						fmt.Printf("        },\n")
					}
					fmt.Printf("        Req: mk%s(),\n", fixName(m.RequestFullType))
					fmt.Printf("        Res: mk%s(),\n", fixName(m.ResponseFullType))
					fmt.Printf("    }\n")
					fmt.Printf("}\n")
				}
			}
		}
	}
}
