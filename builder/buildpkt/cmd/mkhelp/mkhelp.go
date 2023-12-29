package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	gendoc "github.com/pseudomuto/protoc-gen-doc"
)

func prologue(packages map[string]struct{}) {
	fmt.Print(`
package pkthelp

import (
	"google.golang.org/protobuf/proto"
	"github.com/pkt-cash/pktd/btcutil/er"
	"github.com/pkt-cash/pktd/btcutil/util"
`)
	for p := range packages {
		fmt.Println("    " + strconv.Quote(p))
	}
	fmt.Print(`
)

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

var CircularType Type = Type{
	Name: "CIRCULAR",
}
`)
}

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

	openapiFile, err := os.Create("openapi.yaml")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer openapiFile.Close()

	packages := make(map[string]struct{})
	allMessages := make(map[string]string)
	for _, t := range templates {
		for _, f := range t.Files {
			for _, e := range f.Messages {
				if len(strings.Split(e.FullName, ".")) == 2 {
					allMessages[e.FullName] = fixName(e.FullName)
					// "name": "proto/restrpc_pb/external_pb/google_any.proto",
					dir, _ := filepath.Split(f.Name)
					dir = dir[:len(dir)-1] // strip trailing /
					packages["github.com/pkt-cash/pktd/generated/"+dir] = struct{}{}
				}
			}
		}
	}
	prologue(packages)
	schemas := []byte("components:\n    schemas:\n")
	emptyschema := "        EmptyJsonObject:\n            type: object\n            description: An empty JSON\n"
	schemas = append(schemas, []byte(emptyschema)...)

	for _, t := range templates {
		for _, s := range t.Scalars {
			fmt.Printf("func mk%s(_ []string) Type {\n", s.ProtoType)
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
				fmt.Printf("func mk%s(_ []string) Type {\n", fixName(e.FullName))
				fmt.Printf("    return Type{\n")
				fmt.Printf("        Name: %s,\n", strconv.Quote(fixName(e.FullName)))
				desc(e.Description, "        ")
				fmt.Printf("        Fields: []Field{\n")
				schema := "        " + e.FullName + ":\n            type: object\n            properties:\n              enumVarient:\n                type: string\n                enum:\n"
				enums := ""
				for _, v := range e.Values {
					fmt.Printf("            {\n")
					fmt.Printf("                Name: %s,\n", strconv.Quote(v.Name))
					desc(v.Description, "                ")
					fmt.Printf("                Type: EnumVarientType,\n")
					fmt.Printf("            },\n")
					enums += "                - " + v.Name + "\n"
				}
				fmt.Printf("        },\n")
				fmt.Printf("    }\n")
				fmt.Printf("}\n")
				schema += enums

				if !bytes.Contains(schemas, []byte(schema)) {
					schemas = append(schemas, []byte(schema)...)
				}
			}
		}
	}

	for _, t := range templates {
		for _, f := range t.Files {
			// "name": "proto/restrpc_pb/external_pb/google_any.proto",
			for _, e := range f.Messages {
				fmt.Printf("func mk%s(stack []string) Type {\n", fixName(e.FullName))
				fmt.Printf("    if util.Contains(stack, %s) {\n", strconv.Quote(e.FullName))
				fmt.Printf("        return Type {\n")
				fmt.Printf("            Name: %s,\n", strconv.Quote(fixName(e.FullName)))
				fmt.Printf("            Description: []string{\n")
				fmt.Printf("            	\"CIRCULAR REFERENCE, FIELDS OMITTED\",\n")
				fmt.Printf("            },\n")
				fmt.Printf("        }\n")
				fmt.Printf("    }\n")
				fmt.Printf("    stack = append(stack, %s)\n", strconv.Quote(e.FullName))
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
						fmt.Printf("                Type: mk%s(stack),\n", fixName(f.FullType))
						fmt.Printf("            },\n")
					}
					fmt.Printf("        },\n")
				}
				fmt.Printf("    }\n")
				fmt.Printf("}\n")

				// Create OpenAPI schema for this message
				if e.FullName != "google.protobuf.Any" && len(e.Fields) > 0 {
					schema := "        " + e.FullName + ":\n            type: object\n            properties:\n"
					for _, field := range e.Fields {
						description := e.Description
						fieldtype := field.FullType
						ref := ""
						if strings.Contains(fieldtype, ".") {
							ref = "$ref: '#/components/schemas/" + fieldtype + "'"
						} else if fieldtype == "int32" || fieldtype == "int64" || fieldtype == "uint32" || fieldtype == "uint64" || fieldtype == "float" || fieldtype == "double" {
							fieldtype = "integer"
						} else if fieldtype == "bool" {
							fieldtype = "boolean"
						} else if fieldtype == "bytes" {
							fieldtype = "string"
						}
						prop := "              " + field.Name + ":\n                   "
						if ref != "" {
							prop += ref
						} else {
							prop += "type: " + fieldtype
						}
						if e.Description != "" {
							if strings.Contains(description, "\n") {
								description = "|- \n                          " + strings.ReplaceAll(description, "\n", "\n                          ")
							}
							prop += "\n                   description: " + description
						}
						prop += "\n"

						if field.Label == "repeated" {
							prop = "              " + field.Name + ":\n                  type: array\n                  items:\n                    "
							if ref != "" {
								prop += ref
							} else {
								prop += "type: " + fieldtype
							}

							if e.Description != "" {
								prop += "\n                    description: " + description + "\n"
							}
							prop += "\n"
						}
						schema += prop
					}

					if !bytes.Contains(schemas, []byte(schema)) {
						schemas = append(schemas, []byte(schema)...)
					}
				}
			}
		}
	}
	fmt.Printf("func Help(msg proto.Message) (Type, er.R) {\n")
	fmt.Printf("    // If we....\n")
	for k, n := range allMessages {
		fmt.Printf("    if _, ok := msg.(*%s); ok {\n", k)
		fmt.Printf("        return mk%s([]string{}), nil\n", n)
		fmt.Printf("    }\n")
	}
	fmt.Printf("    var ret Type\n")
	fmt.Printf("    return ret, er.Errorf(\"no help for type [%%T]\", msg)\n")
	fmt.Printf("}\n")

	var categoryRegexp *regexp.Regexp
	var shortDescriptionRegexp *regexp.Regexp

	categoryRegexp, err = regexp.Compile("\\$pld\\.category:\\s*`([^`]+)`")
	if err != nil {
		panic(err.Error())
	}

	shortDescriptionRegexp, err = regexp.Compile("\\$pld\\.short_description:\\s*`([^`]+)`")
	if err != nil {
		panic(err.Error())
	}

	openapiFile.Write([]byte("# Generated with pktd mkhelp\n"))
	openapiFile.Write([]byte("openapi: 3.0.3\n"))
	openapiFile.Write([]byte("info:\n"))
	openapiFile.Write([]byte("    title: PKT Lightning Deamon API\n"))
	openapiFile.Write([]byte("    description: PKT Lightning Deamon REST API.\n"))
	openapiFile.Write([]byte("    version: 0.1.1\n"))
	openapiFile.Write([]byte("    x-tagGroups:\n"))
	openapiFile.Write([]byte("        - name: Lightning Service\n"))
	openapiFile.Write([]byte("          tags:\n"))
	openapiFile.Write([]byte("              - Lightning\n"))
	openapiFile.Write([]byte("              - Lightning Autopilot\n"))
	openapiFile.Write([]byte("              - Lightning Channel\n"))
	openapiFile.Write([]byte("              - Lightning Graph\n"))
	openapiFile.Write([]byte("              - Lightning Invoice\n"))
	openapiFile.Write([]byte("              - Lightning Payment\n"))
	openapiFile.Write([]byte("              - Lightning Peer\n"))
	openapiFile.Write([]byte("              - Lightning Watchtower\n"))
	openapiFile.Write([]byte("        - name: Meta Service\n"))
	openapiFile.Write([]byte("          tags:\n"))
	openapiFile.Write([]byte("              - MetaService\n"))
	openapiFile.Write([]byte("        - name: Utilities Service\n"))
	openapiFile.Write([]byte("          tags:\n"))
	openapiFile.Write([]byte("              - Utilities\n"))
	openapiFile.Write([]byte("        - name: Wallet Service\n"))
	openapiFile.Write([]byte("          tags:\n"))
	openapiFile.Write([]byte("              - Wallet\n"))
	openapiFile.Write([]byte("              - Wallet Unspent\n"))
	openapiFile.Write([]byte("              - Wallet Address\n"))
	openapiFile.Write([]byte("              - Wallet Transaction\n"))
	openapiFile.Write([]byte("        - name: Neutrino Service\n"))
	openapiFile.Write([]byte("          tags:\n"))
	openapiFile.Write([]byte("              - Neutrino\n"))
	openapiFile.Write([]byte("        - name: Cjdns\n"))
	openapiFile.Write([]byte("          tags:\n"))
	openapiFile.Write([]byte("              - Cjdns\n\n"))

	apipaths := []byte("paths:\n")
	for _, t := range templates {
		for _, f := range t.Files {
			for _, s := range f.Services {
				for _, m := range s.Methods {
					fmt.Printf("func %s_%s() Method {\n", s.Name, m.Name)
					fmt.Printf("    return Method{\n")
					fmt.Printf("        Name: %s,\n", strconv.Quote(m.Name))
					fmt.Printf("        Service: %s,\n", strconv.Quote(s.Name))

					// if s.Name == "Autopilot" {
					// 	apipath = (fmt.Sprintf("  /v1/lightning/%s/%s:\n", strings.ToLower(s.Name), strings.ToLower(m.Name)))
					// } else if s.Name == "Invoices" {
					// 	apipath = (fmt.Sprintf("  /v1/lightning/%s/%s:\n", strings.ToLower(s.Name), strings.ToLower(m.Name)))
					// } else if s.Name == "Invoices" {
					// 	apipath = (fmt.Sprintf("  /v1/lightning/%s/%s:\n", strings.ToLower(s.Name), strings.ToLower(m.Name)))
					// } else {

					// }
					apipath := (fmt.Sprintf("  /v1/%s/%s:\n", strings.ToLower(s.Name), strings.ToLower(m.Name)))

					if len(s.Description) > 0 {

						var match []string
						var matchIndex []int

						match = categoryRegexp.FindStringSubmatch(m.Description)
						if len(match) > 1 {
							fmt.Printf("        Category: %s,\n", strconv.Quote(match[1]))
							apipath = (fmt.Sprintf("  /v1/%s/%s:\n", strings.ToLower(match[1]), strings.ToLower(m.Name)))
							matchIndex = categoryRegexp.FindStringIndex(m.Description)
							m.Description = m.Description[0:matchIndex[0]] + m.Description[matchIndex[1]:]
						}

						match = shortDescriptionRegexp.FindStringSubmatch(m.Description)
						// shortDescription := " \"\""
						if len(match) > 1 {
							fmt.Printf("        ShortDescription: %s,\n", strconv.Quote(match[1]))
							// shortDescription = strings.ReplaceAll(match[1], ":", " ")
							matchIndex = shortDescriptionRegexp.FindStringIndex(m.Description)
							m.Description = m.Description[0:matchIndex[0]] + m.Description[matchIndex[1]:]
						}

						fmt.Printf("        Description: []string{\n")
						//TODO:get or post?
						apipath += "    post:\n"
						//apipath += "        summary: " + shortDescription + "\n"
						apipath += "        summary: " + m.Name + "\n"
						apipath += "        tags:\n"
						apipath += fmt.Sprintf("            - %s\n", s.Name)
						apipath += "        description: |-\n"
						description := strings.Trim(m.Description, "\n")
						description = strings.ReplaceAll(description, "\n", "\n            ")
						apipath += fmt.Sprintf("            %s\n", description)
						apipath += fmt.Sprintf("        operationId: %s\n", m.Name)
						for _, s := range strings.Split(m.Description, "\n") {
							descriptionLine := strings.TrimSpace(s)
							if len(descriptionLine) > 0 {
								fmt.Printf("            %s,\n", strconv.Quote(descriptionLine))
							}
						}
						fmt.Printf("        },\n")
					}
					fmt.Printf("        Req: mk%s([]string{}),\n", fixName(m.RequestFullType))
					apipath += "        requestBody:\n"
					apipath += "            content:\n"
					apipath += "                application/json:\n"
					apipath += "                    schema:\n"
					apipath += fmt.Sprintf("                        $ref: '#/components/schemas/%s'\n", m.RequestFullType)
					fmt.Printf("        Res: mk%s([]string{}),\n", fixName(m.ResponseFullType))
					apipath += "        responses:\n"
					apipath += "            '200':\n"
					apipath += "                description: Successful operation\n"
					apipath += "                content:\n"
					apipath += "                    application/json:\n"
					apipath += "                        schema:\n"
					apipath += fmt.Sprintf("                            $ref: '#/components/schemas/%s'\n", m.ResponseFullType)
					apipaths = append(apipaths, []byte(apipath)...)
					fmt.Printf("    }\n")
					fmt.Printf("}\n")
				}
			}
		}
	}

	// Write schema to openapiFile
	spaths := string(apipaths)

	// Replace empty json objects in openapi
	pattern := `(\$ref: '#/components/schemas/[^']+')`
	re := regexp.MustCompile(pattern)

	matches := re.FindAllStringSubmatch(spaths, -1)
	for _, match := range matches {
		// Extract the substring after the last "/"
		schemaName := extractSchemaName(match[1])
		schemaName = strings.ReplaceAll(schemaName, "'", "")
		println("Schema: " + schemaName)
		if !strings.Contains(string(schemas), schemaName) {
			// Replace the entire "$ref" with "{}"
			println("NOT found " + schemaName + " in schemas ")
			spaths = strings.Replace(spaths, match[0], "{}", 1)
		}
	}
	openapiFile.WriteString(spaths)
	openapiFile.WriteString(string(schemas))
}

func extractSchemaName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}
