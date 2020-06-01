package injector

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestPatchYaml(t *testing.T) {
	yamlString := `
filebeat.inputs:
- type: log
  enabled: true
  paths:
  - /data/log/*.log
output.console:
  codec.format:
    string: '%{[message]}'
logging.level: warning
`
	patchJsonString := `[
{"op":"add","path":"/filebeat.inputs/0/tail_files","value":true}
]`
	newYamlString, err := PatchYaml(yamlString, patchJsonString)
	if err != nil {
		panic(err)
	}
	if !strings.Contains(newYamlString, "- tail_files: true") &&
		!strings.Contains(newYamlString, "  tail_files: true") {
		assert.Fail(t, "failed to patch yaml")
	}
}
