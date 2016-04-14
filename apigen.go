package apigen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	//	"golang.org/x/tools/imports"
	"os"
	//	"strconv"
	"path/filepath"
	"strings"
	"text/template"
	"unicode"
)

const usage = `apigen def.go
`

type TemplateData struct {
	Global     *GlobalConfig
	Definition *DefConfig
}

type GlobalConfig struct {
	Globals   map[string]string
	Returns   map[string]string
	Arguments map[string]string
	Templates map[string]*template.Template
	Options   map[string]string
}

type DefConfig struct {
	Gen       string
	Method    string
	OnType    []string
	Alias     string
	Arguments string
	Return    string
	Options   map[string]string
	Template  string
	Ref       *APIRef
}

type Generator interface {
	OnDefine(config GlobalConfig, def DefConfig) (string, error)
}

func Process() {
	var config = GlobalConfig{}
	config.Options = make(map[string]string)
	config.Returns = make(map[string]string)
	config.Arguments = make(map[string]string)
	config.Globals = make(map[string]string)
	config.Templates = make(map[string]*template.Template)
	fmt.Println("%d", len(os.Args))
	if len(os.Args) != 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	defFile := os.Args[1]
	if _, err := os.Stat(defFile); os.IsNotExist(err) {
		fmt.Println("File %s could not be found", defFile)
		os.Exit(1)
	}

	defs := config.parseDefs(defFile)
	for _, def := range defs {
		def.setup(&config)
		def.output(&config)
	}
}

func (config *GlobalConfig) parseDefs(file string) []*DefConfig {
	fset := token.NewFileSet() // positions are relative to fset

	// Parse the file containing this very example
	// but stop after processing the imports.
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	var defs = make([]*DefConfig, 0)
	// Print the imports from the file's AST.
	for _, s := range f.Comments {
		for _, line := range strings.Split(s.Text(), "\n") {
			//fmt.Println("Comment: %s", line)
			def := parseLine(config, line)
			if def != nil {
				defs = append(defs, def)
			}
		}
	}
	return defs
}

func parseLine(config *GlobalConfig, line string) *DefConfig {
	line = strings.TrimSpace(line)
	lastQuote := rune(0)
	lastChar := rune(0)
	f := func(c rune) bool {
		switch {
		case c == lastQuote:
			lastQuote = rune(0)
			return false
		case lastQuote != rune(0):
			return false
		case unicode.In(c, unicode.Quotation_Mark):
			lastQuote = c
			return false
		case lastChar == ':' && unicode.IsSpace(c):
			return false
		default:
			lastChar = c
			return unicode.IsSpace(c)

		}
	}
	fields := strings.FieldsFunc(line, f)
	if len(fields) > 0 && fields[0] == "apig" {
		var def *DefConfig
		def = &DefConfig{}

		for i := 1; i < len(fields); i++ {
			sep := strings.Index(fields[i], ":")
			//fmt.Printf("Split: %d == %s\n", sep, fields[i])
			fieldName := strings.TrimSpace(fields[i][0:sep])
			fieldValue := strings.TrimSpace(fields[i][sep+1:])
			fieldExtra := ""
			specials := [...]string{"Args", "Template", "Return", "All"}
			for _, special := range specials {
				if sep := strings.LastIndex(fieldName, special); sep > 0 {
					fieldExtra = fieldName[0:sep]
					fieldName = special
					break
				}
			}

			switch fieldName {
			case "All":
				if fieldExtra != "" {
					config.Options[fieldExtra] = fieldValue
				}
			case "Args":
				if fieldExtra != "" {
					config.Arguments[fieldExtra] = fieldValue
				}
			case "Return":
				if fieldExtra != "" {
					config.Returns[fieldExtra] = fieldValue
				}
			case "Template":
				if fieldExtra != "" {
					err := config.processTemplateFile(fieldExtra, fieldValue)
					if err != nil {
						fmt.Printf("Failed to process template: %v", err)
					}
				}
			case "args":
				def.Arguments = fieldValue
			case "gen":
				def.Gen = fieldValue
			case "alias":
				def.Alias = fieldValue
			case "return":
				def.Return = fieldValue
			case "template":
				def.Template = fieldValue
			default:
				if def.Options == nil {
					def.Options = make(map[string]string)
				}
				def.Options[fieldName] = fieldValue
			}
			fmt.Printf("Field %s(%s) == %s\n", fieldName, fieldExtra, fieldValue)
		}
		if def.Gen != "" {
			return def
		}
	}
	return nil
}

func (def *DefConfig) setup(config *GlobalConfig) {
	val, ok := config.Arguments[def.Arguments]
	if ok {
		def.Arguments = strings.Replace(val, "\"", "", -1)
	}
	val, ok = config.Returns[def.Return]
	if ok {
		def.Return = val
	}

	ref, err := findImport(def.Gen)
	if err != nil {
		fmt.Printf("%v", err)
		return
	}

	if idx := strings.Index(def.Gen, "->"); idx > 0 {
		def.OnType = strings.Split(def.Gen[0:idx], ",")
		def.Method = def.Gen[idx+2:]
	} else {
		def.Method = def.Gen
	}

	def.Ref = ref
}

func (def *DefConfig) GetArguments() map[string]interface{} {
	return nil
}

func (def *DefConfig) output(config *GlobalConfig) {
	var data = TemplateData{
		Global:     config,
		Definition: def,
	}
	var buffer bytes.Buffer
	err := config.Templates[def.Template].Execute(&buffer, data)

	if err != nil {
		fmt.Printf("Error occured processing template: %v", err)
	}
	fmt.Printf("Template: %s\n", buffer.String())
}

func (config *GlobalConfig) processTemplateFile(name string, value string) error {
	var tmpl *template.Template

	// Template files can be absolute or live locally under the
	// configuration directory in /tpl
	if _, err := os.Stat(value); err != nil {
		return err
	}
	tmpl = template.Must(template.New(value).ParseFiles(value))

	// See above
	tmpl.Option("missingkey=error")
	config.Templates[name] = tmpl
	return nil
}

type APIRef struct {
	Pkg       string
	Call      string
	Returns   []*FieldInfo
	Arguments []*FieldInfo
	Receive   []*FieldInfo
}

func findImport(value string) (ref *APIRef, err error) {
	funcName := value[strings.LastIndex(value, ".")+1:]
	var funcType []string
	if idx := strings.Index(funcName, "->"); idx > 0 {
		funcType = strings.Split(funcName[0:idx], ",")
		funcName = funcName[idx+2:]
	}
	pkg, err := build.Import(value[0:strings.LastIndex(value, ".")], "", 0)

	fset := token.NewFileSet() // share one fset across the whole package
	for _, file := range pkg.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(pkg.Dir, file), nil, 0)
		if err != nil {
			continue
		}

		for _, decl := range f.Decls {
			decl, ok := decl.(*ast.FuncDecl)

			if !ok || decl.Name.Name != funcName {
				continue
			}
			ref := APIRef{}

			if decl.Recv != nil {
				if len(funcType) == 0 {
					continue
				}
				ref.Receive = make([]*FieldInfo, len(decl.Recv.List), len(decl.Recv.List))
				for idx, param := range decl.Recv.List {
					fInfo, err := getFieldInfo(param)
					if err != nil {
						return nil, err
					}
					ref.Receive[idx] = fInfo
				}
			}
			if len(funcType) > 0 {
				if len(funcType) != len(ref.Receive) {
					continue
				}
				isMatch := true
				for idx, fi := range ref.Receive {
					if funcType[idx] != fi.Type {
						isMatch = false
						break
					}
				}
				if !isMatch {
					continue
				}
			}

			ref.Pkg = pkg.Name
			ref.Call = funcName

			fmt.Printf("Data: %v %v\n", decl, decl.Type.Params)
			if decl.Type.Params != nil {
				ref.Arguments = make([]*FieldInfo, len(decl.Type.Params.List), len(decl.Type.Params.List))
				for idx, param := range decl.Type.Params.List {
					fmt.Printf("InnerParam %v\n", param)
					fInfo, err := getFieldInfo(param)
					if err != nil {
						return nil, err
					}
					ref.Arguments[idx] = fInfo
					fmt.Printf("Param: %s \n", fInfo)
				}
			}

			if decl.Recv != nil {
				ref.Receive = make([]*FieldInfo, len(decl.Recv.List), len(decl.Recv.List))
				for idx, param := range decl.Recv.List {
					fInfo, err := getFieldInfo(param)
					if err != nil {
						return nil, err
					}
					ref.Receive[idx] = fInfo

				}
			}

			if decl.Type.Results != nil {
				ref.Returns = make([]*FieldInfo, len(decl.Type.Results.List), len(decl.Type.Results.List))
				for idx, param := range decl.Type.Results.List {
					fInfo, err := getFieldInfo(param)
					if err != nil {
						return nil, err
					}
					ref.Returns[idx] = fInfo
				}
			}
			return &ref, nil
		}

	}
	//return Pkg{}, nil, fmt.Errorf("type %s not found in %s", id, path)

	return nil, fmt.Errorf("Could not locate reference: %s", value)
}

type FieldInfo struct {
	Name    string
	Type    string
	IsError bool
}

func getFieldInfo(field *ast.Field) (*FieldInfo, error) {
	fInfo := FieldInfo{}
	if len(field.Names) > 0 {
		fInfo.Name = field.Names[0].Name
	}
	if fType, ok := field.Type.(*ast.ArrayType); ok {
		subType, err := getFieldType(fType.Elt)
		if err != nil {
			return nil, err
		}
		fInfo.Type = "[]" + subType
		return &fInfo, nil
	}
	if fType, ok := field.Type.(*ast.StarExpr); ok {
		fInfo.Type = "*" + fType.X.(*ast.Ident).Name
		return &fInfo, nil
	}
	if fType, ok := field.Type.(*ast.Ident); ok {
		fInfo.Type = fType.Name
		if fType.Name == "error" {
			fInfo.IsError = true
		}
		return &fInfo, nil
	}
	return nil, fmt.Errorf("Unexpected field type")
}

func getFieldType(field ast.Expr) (string, error) {

	if fType, ok := field.(*ast.ArrayType); ok {
		subType, err := getFieldType(fType.Elt)
		if err != nil {
			return "", err
		}
		typeInfo := "[]" + subType
		return typeInfo, nil
	}
	if fType, ok := field.(*ast.StarExpr); ok {
		typeInfo := "*" + fType.X.(*ast.Ident).Name
		return typeInfo, nil
	}
	if fType, ok := field.(*ast.Ident); ok {
		typeInfo := fType.Name
		return typeInfo, nil
	}
	return "", fmt.Errorf("Unexpected field type")
}

func (f FieldInfo) String() string {
	if f.Name == "" {
		return f.Type
	}
	return fmt.Sprintf("%s %s", f.Name, f.Type)
}
