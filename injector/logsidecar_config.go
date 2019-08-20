package injector

import (
	"encoding/json"
	"k8s.io/apimachinery/pkg/api/resource"
	"strings"
)

var (
	lscCpuLimit    = resource.MustParse("100m")
	lscMemoryLimit = resource.MustParse("100Mi")

	lscCpuRequest    = resource.MustParse("10m")
	lscMemoryRequest = resource.MustParse("10Mi")
)

type LSCConfig struct {
	ContainerLogConfigs ContainerLogConfigs `json:"containerLogConfigs,omitempty"`
}
type ContainerLogConfigs map[string]VolumeLogConfig // key: containerName; value: VolumeLogConfig
type VolumeLogConfig map[string][]string            // key: volumeName; value: logRelPaths

func decodeLSCConfig(confStr string) (*LSCConfig, error) {
	confStr = strings.TrimSpace(confStr)
	if confStr != "" {
		conf := &LSCConfig{}
		err := json.Unmarshal([]byte(confStr), conf)
		if err != nil {
			conf = nil
		}
		return conf, err
	}
	return nil, nil
}
