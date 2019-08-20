package injector

import (
	"bytes"
	"fmt"
)

const (
	FilebeatConfDir       = "/etc/logsidecar"
	FilebeatYMLPath       = FilebeatConfDir + "/filebeat.yml"
	FilebeatInputsYMLPath = FilebeatConfDir + "/inputs.yml"
)

// for init filebeat.yml
var fileBeatYMLInitCMD string
var filebeatConfLines = []string{
	"filebeat.config.inputs:",
	"  enabled: true",
	"  path: \\${path.config}/inputs.yml",
	"  reload.enabled: true",
	"  reload.period: 10s",
	"output.console:",
	"  codec.format:",
	"    string: '%{[message]}'",
	"logging.level: warning",
}

// for init inputs.yml
var filebeatInputsHeadLines = []string{
	"- type: log",
	"  paths:"}

func init() {
	fileBeatYMLInitCMD = generateEchoCMD(filebeatConfLines, FilebeatYMLPath)
}

func filebeatInputsYMLInitCMD(logAbsPaths []string) string {
	confLines := filebeatInputsHeadLines[:]
	for _, path := range logAbsPaths {
		confLines = append(confLines, "  - "+path)
	}
	return generateEchoCMD(confLines, FilebeatInputsYMLPath)
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

func FilebeatConfigInitCMD(logAbsPaths []string) string {
	dirReadyCMD := fmt.Sprintf(
		"if [ ! -d %s ];then mkdir %s;fi;", FilebeatConfDir, FilebeatConfDir)
	YMLReadyCMD := fmt.Sprintf(
		"if [ -e %s ];then >%s;fi;", FilebeatYMLPath, FilebeatYMLPath)
	InputsYMLReadyCMD := fmt.Sprintf(
		"if [ -e %s ];then >%s;fi;", FilebeatInputsYMLPath, FilebeatInputsYMLPath)
	var buffer bytes.Buffer
	buffer.WriteString(dirReadyCMD)
	buffer.WriteString(YMLReadyCMD)
	buffer.WriteString(InputsYMLReadyCMD)
	buffer.WriteString(fileBeatYMLInitCMD)
	buffer.WriteString(filebeatInputsYMLInitCMD(logAbsPaths))
	return buffer.String()
}
