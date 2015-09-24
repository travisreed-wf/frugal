package dartlang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"gopkg.in/yaml.v2"

	"github.com/Workiva/frugal/compiler/generator"
	"github.com/Workiva/frugal/compiler/globals"
	"github.com/Workiva/frugal/compiler/parser"
)

const (
	lang             = "dart"
	defaultOutputDir = "gen-dart"
)

type Generator struct {
	*generator.BaseGenerator
}

func NewGenerator() generator.MultipleFileGenerator {
	return &Generator{&generator.BaseGenerator{}}
}

func getPackageComponents(pkg string) []string {
	return strings.Split(pkg, ".")
}

func (g *Generator) GetOutputDir(dir string, p *parser.Program) string {
	if pkg, ok := p.Namespaces[lang]; ok {
		path := getPackageComponents(pkg)
		dir = filepath.Join(append([]string{dir}, path...)...)
	} else {
		dir = filepath.Join(dir, p.Name)
	}
	return dir
}

func (g *Generator) DefaultOutputDir() string {
	return defaultOutputDir
}

func (g *Generator) GenerateDependencies(p *parser.Program, dir string) error {
	if err := g.addToPubspec(p, dir); err != nil {
		return err
	}
	if err := g.exportClasses(p, dir); err != nil {
		return err
	}
	return nil
}

type pubspec struct {
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	Description  string `yaml:"description"`
	Environment  env    `yaml:"environment"`
	Dependencies deps   `yaml:"dependencies"`
}

type env struct {
	SDK string `yaml:"sdk"`
}

type deps struct {
	Thrift dep `yaml:"thrift"`
	Frugal dep `yaml:"frugal"`
}

type dep struct {
	Git gitDep `yaml:"git"`
}

type gitDep struct {
	URL string `yaml:"url"`
}

func (g *Generator) addToPubspec(p *parser.Program, dir string) error {
	pubFilePath := filepath.Join(dir, "pubspec.yaml")
	ps := &pubspec{
		Name:        strings.ToLower(p.Name),
		Version:     "0.0.1",
		Description: "Autogenerated by the frugal compiler",
		Environment: env{
			SDK: "^1.12.0",
		},
		Dependencies: deps{
			Thrift: dep{
				Git: gitDep{
					URL: "git@github.com:Workiva/thrift-dart.git",
				},
			},
			Frugal: dep{
				Git: gitDep{
					URL: "git@github.com:Workiva/frugal-dart.git",
				},
			},
		},
	}

	d, err := yaml.Marshal(&ps)
	if err != nil {
		return err
	}
	// create and write to new file
	newPubFile, err := os.Create(pubFilePath)
	defer newPubFile.Close()
	if err != nil {
		return nil
	}
	if _, err := newPubFile.Write(d); err != nil {
		return err
	}
	return nil
}

func (g *Generator) exportClasses(p *parser.Program, dir string) error {
	dartFile := fmt.Sprintf("%s.%s", strings.ToLower(p.Name), lang)
	mainFilePath := filepath.Join(dir, "lib", dartFile)
	mainFile, err := os.OpenFile(mainFilePath, syscall.O_RDWR, 0777)
	defer mainFile.Close()
	if err != nil {
		return err
	}

	exports := "\n"
	for _, scope := range p.Scopes {
		exports += fmt.Sprintf("export 'src/%s%s.%s' show %sPublisher, %sSubscriber;\n",
			generator.FilePrefix, strings.ToLower(scope.Name), lang, scope.Name, scope.Name)
	}
	stat, err := mainFile.Stat()
	if err != nil {
		return err
	}
	_, err = mainFile.WriteAt([]byte(exports), stat.Size())
	return err
}

func (g *Generator) CheckCompile(path string) error {
	// TODO: Add compile to js
	return nil
}

func (g *Generator) GenerateFile(name, outputDir string, fileType generator.FileType) (*os.File, error) {
	if fileType != generator.CombinedFile {
		return nil, fmt.Errorf("frugal: Bad file type for dartlang generator")
	}
	outputDir = filepath.Join(outputDir, "lib")
	outputDir = filepath.Join(outputDir, "src")
	return g.CreateFile(name, outputDir, lang)
}

func (g *Generator) GenerateDocStringComment(file *os.File) error {
	comment := fmt.Sprintf(
		"// Autogenerated by Frugal Compiler (%s)\n"+
			"// DO NOT EDIT UNLESS YOU ARE SURE THAT YOU KNOW WHAT YOU ARE DOING",
		globals.Version)

	_, err := file.WriteString(comment)
	return err
}

func (g *Generator) GeneratePackage(file *os.File, p *parser.Program, scope *parser.Scope) error {
	// TODO: Figure out what this does
	pkg, ok := p.Namespaces[lang]
	if ok {
		components := getPackageComponents(pkg)
		pkg = components[len(components)-1]
	} else {
		pkg = p.Name
	}
	_, err := file.WriteString(fmt.Sprintf("library %s.src.%s%s;", pkg,
		generator.FilePrefix, strings.ToLower(scope.Name)))
	return err
}

func (g *Generator) GenerateImports(file *os.File, scope *parser.Scope) error {
	imports := "import 'dart:async';\n"
	imports += "import 'package:thrift/thrift.dart' as thrift;\n"
	imports += "import 'package:frugal/frugal.dart' as frugal;\n"
	params := map[string]bool{}
	for _, op := range scope.Operations {
		if _, ok := params[op.Param]; !ok {
			params[op.Param] = true
		}
	}
	for key, _ := range params {
		lowerKey := strings.ToLower(key)
		imports += fmt.Sprintf("import '%s.dart' as t_%s;\n", lowerKey, lowerKey)
	}
	_, err := file.WriteString(imports)
	return err
}

func (g *Generator) GenerateConstants(file *os.File, name string) error {
	constants := fmt.Sprintf("const String delimiter = '%s';", globals.TopicDelimiter)
	_, err := file.WriteString(constants)
	return err
}

func (g *Generator) GeneratePublisher(file *os.File, scope *parser.Scope) error {
	publishers := ""
	publishers += fmt.Sprintf("class %sPublisher {\n", scope.Name)
	publishers += "\tfrugal.Transport transport;\n"
	publishers += "\tthrift.TProtocol protocol;\n"
	publishers += "\tint seqId;\n\n"

	publishers += fmt.Sprintf("\t%sPublisher(frugal.TransportFactory t, thrift.TTransportFactory f, "+
		"thrift.TProtocolFactory p) {\n", scope.Name)
	publishers += "\t\tvar provider = new frugal.Provider(t, f, p);\n"
	publishers += "\t\tvar tp = provider.newTransportProtocol();\n"
	publishers += "\t\ttransport = tp.transport;\n"
	publishers += "\t\tprotocol = tp.protocol;\n"
	publishers += "\t\tseqId = 0;\n"
	publishers += "\t}\n\n"

	prefix := ""
	// TODO: Add args
	for _, op := range scope.Operations {
		publishers += prefix
		prefix = "\n\n"
		publishers += fmt.Sprintf("\tpublish%s(t_%s.%s req) {\n", op.Name, strings.ToLower(op.Param), op.Param)
		publishers += fmt.Sprintf("\t\tvar op = \"%s\";\n", op.Name)
		publishers += fmt.Sprintf("\t\tvar prefix = \"%s\";\n", generatePrefixStringTemplate(scope))
		publishers += "\t\tvar topic = \"${prefix}" + scope.Name + "${delimiter}${op}\";\n"
		publishers += "\t\ttransport.preparePublish(topic);\n"
		publishers += "\t\tvar oprot = protocol;\n"
		publishers += "\t\tseqId++;\n"
		publishers += "\t\tvar msg = new thrift.TMessage(op, thrift.TMessageType.CALL, seqId);\n"
		publishers += "\t\toprot.writeMessageBegin(msg);\n"
		publishers += "\t\treq.write(oprot);\n"
		publishers += "\t\toprot.writeMessageEnd();\n"
		publishers += "\t\treturn oprot.transport.flush();\n"
		publishers += "\t}\n"
	}

	publishers += "}\n"

	_, err := file.WriteString(publishers)
	return err
}

func generatePrefixStringTemplate(scope *parser.Scope) string {
	if len(scope.Prefix.Variables) == 0 {
		return ""
	}
	template := "fmt.Sprintf(\""
	template += scope.Prefix.Template()
	template += globals.TopicDelimiter + "\", "
	prefix := ""
	for _, variable := range scope.Prefix.Variables {
		template += prefix + variable
		prefix = ", "
	}
	template += ")"
	return template
}

func (g *Generator) GenerateSubscriber(file *os.File, scope *parser.Scope) error {
	subscribers := ""
	subscribers += fmt.Sprintf("class %sSubscriber {\n", scope.Name)
	subscribers += "\tfrugal.Provider provider;\n\n"

	subscribers += fmt.Sprintf("\t%sSubscriber(frugal.TransportFactory t, thrift.TTransportFactory f, "+
		"thrift.TProtocolFactory p) {\n", scope.Name)
	subscribers += "\t\tprovider = new frugal.Provider(t, f, p);\n"
	subscribers += "\t}\n\n"

	prefix := ""
	// TODO: Add args
	for _, op := range scope.Operations {
		paramLower := strings.ToLower(op.Param)
		subscribers += prefix
		prefix = "\n\n"
		subscribers += fmt.Sprintf("\tsubscribe%s(dynamic on%s(t_%s.%s req)) async {\n",
			op.Name, op.Param, paramLower, op.Param)
		subscribers += fmt.Sprintf("\t\tvar op = \"%s\";\n", op.Name)
		subscribers += fmt.Sprintf("\t\tvar prefix = \"%s\";\n", generatePrefixStringTemplate(scope))
		subscribers += "\t\tvar topic = \"${prefix}" + scope.Name + "${delimiter}${op}\";\n"
		subscribers += "\t\tvar tp = provider.newTransportProtocol();\n"
		subscribers += "\t\tawait tp.transport.subscribe(topic);\n"
		subscribers += "\t\ttp.transport.signalRead.listen((_) {\n"
		subscribers += fmt.Sprintf("\t\t\ton%s(_recv%s(op, tp.protocol));\n", op.Param, op.Name)
		subscribers += "\t\t});\n"
		subscribers += "\t\treturn new frugal.Subscription(topic, tp.transport);\n"
		subscribers += "\t}\n\n"

		subscribers += fmt.Sprintf("\tt_%s.%s _recv%s(String op, thrift.TProtocol iprot) {\n",
			paramLower, op.Param, op.Name)
		subscribers += "\t\tvar tMsg = iprot.readMessageBegin();\n"
		subscribers += "\t\tif (tMsg.name != op) {\n"
		subscribers += "\t\t\tthrift.TProtocolUtil.skip(iprot, thrift.TType.STRUCT);\n"
		subscribers += "\t\t\tiprot.readMessageEnd();\n"
		subscribers += "\t\t\tthrow new thrift.TApplicationError(\n"
		subscribers += "\t\t\t\tthrift.TApplicationErrorType.UNKNOWN_METHOD, tMsg.name);\n"
		subscribers += "\t\t}\n"
		subscribers += fmt.Sprintf("\t\tvar req = new t_%s.%s();\n", paramLower, op.Param)
		subscribers += "\t\treq.read(iprot);\n"
		subscribers += "\t\tiprot.readMessageEnd();\n"
		subscribers += "\t\treturn req;\n"
		subscribers += "\t}\n"
	}

	subscribers += "}\n"

	_, err := file.WriteString(subscribers)
	return err
}
