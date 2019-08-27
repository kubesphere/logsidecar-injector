package injector

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"
)

const (
	FilebeatConfDir       = "/etc/logsidecar"
	FilebeatYMLPath       = FilebeatConfDir + "/filebeat.yml"
	FilebeatInputsYMLPath = FilebeatConfDir + "/inputs.yml"
)

var filebeatYMLTmpl, filebeatInputsYMLTmpl string

var filebeatInputsDataTmpl *template.Template

type FilebeatInputsData struct {
	Paths []string
}

func AddFilebeatTmplFlags() {
	flag.StringVar(&filebeatYMLTmpl, "filebeat-yml-template", filebeatYMLTmpl, "template for filebeat.yml")
	flag.StringVar(&filebeatInputsYMLTmpl, "inputs-yml-template", filebeatInputsYMLTmpl, "template for inputs.yml")
}

var filebeatYMLInitCMD string

func InitFilebeatTmpl() {
	// generate filebeat.yml init cmd
	f, err := os.Open(filebeatYMLTmpl)
	if err != nil {
		panic(fmt.Errorf("error to open filebeat-yml-template file: %v", err))
	}
	var lines []string
	r, e := regexp.Compile(`(^|^.*[^\$]{1})(\$)($|[^\$]{1}.*)`) // used to replace single $
	if e != nil {
		panic(e)
	}
	for b := bufio.NewScanner(f); b.Scan(); {
		if s := b.Text(); strings.TrimSpace(s) != "" {
			lines = append(lines, r.ReplaceAllString(s, "$1\\$2$3"))
		}
	}
	filebeatYMLInitCMD = generateEchoCMD(lines, FilebeatYMLPath)
	// generate template for inputs.yml
	if filebeatInputsDataTmpl, err = template.ParseFiles(filebeatInputsYMLTmpl); err != nil {
		panic(fmt.Errorf("error to create tempalte from inputs-yml-template file: %v", err))
	}
}

func filebeatInputsYMLInitCMD(logAbsPaths []string) (string, error) {
	buffer := bytes.NewBufferString("")
	if err := filebeatInputsDataTmpl.Execute(buffer, &FilebeatInputsData{Paths: logAbsPaths}); err != nil {
		return "", err
	}
	var lines []string
	for b := bufio.NewScanner(buffer); b.Scan(); {
		if s := b.Text(); strings.TrimSpace(s) != "" {
			lines = append(lines, s)
		}
	}
	return generateEchoCMD(lines, FilebeatInputsYMLPath), nil
}

func generateEchoCMD(lines []string, filePath string) string {
	var buffer bytes.Buffer
	for _, c := range lines {
		buffer.WriteString("echo \"")
		buffer.WriteString(c)
		buffer.WriteString("\" >> ")
		buffer.WriteString(filePath)
		buffer.WriteString(";")
	}
	return buffer.String()
}

func FilebeatConfigInitCMD(logAbsPaths []string) (string, error) {
	clearCMD := fmt.Sprintf(
		"if [ -e %s ];then >%s;fi;", FilebeatYMLPath, FilebeatYMLPath)
	inputsClearCMD := fmt.Sprintf(
		"if [ -e %s ];then >%s;fi;", FilebeatInputsYMLPath, FilebeatInputsYMLPath)
	inputsYMLInitCMD, err := filebeatInputsYMLInitCMD(logAbsPaths)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	buffer.WriteString(clearCMD)
	buffer.WriteString(inputsClearCMD)
	buffer.WriteString(filebeatYMLInitCMD)
	buffer.WriteString(inputsYMLInitCMD)
	return buffer.String(), nil
}
