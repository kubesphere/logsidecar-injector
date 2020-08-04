package injector

import (
	"bufio"
	"bytes"
	"sigs.k8s.io/yaml"
	"strings"

	"github.com/evanphx/json-patch"
)

func JoinLines(lines string, addLinePrefix, addLineSuffix string) string {
	joins := bytes.NewBufferString(lines)
	var sb strings.Builder
	for b := bufio.NewScanner(joins); b.Scan(); {
		if s := b.Text(); strings.TrimSpace(s) != "" {
			sb.WriteString(addLinePrefix)
			sb.WriteString(s)
			sb.WriteString(addLineSuffix)
		}
	}
	return sb.String()
}

func PatchYaml(yamlString, patchJsonString string) (string, error) {
	patch, err := jsonpatch.DecodePatch([]byte(patchJsonString))
	if err != nil {
		return "", err
	}
	jsonBytes, err := yaml.YAMLToJSONStrict([]byte(yamlString))
	if err != nil {
		return "", err
	}
	newJsonBytes, err := patch.Apply(jsonBytes)
	if err != nil {
		return "", err
	}
	newYamlBytes, err := yaml.JSONToYAML(newJsonBytes)
	if err != nil {
		return "", err
	}
	return string(newYamlBytes), nil
}
