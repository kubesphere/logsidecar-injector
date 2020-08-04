package injector

import (
	"encoding/json"
	"strings"
	"sync"
)

var (
	injectorConfig *InjectorConfig
	mutex          sync.Mutex
)

func ReloadInjectorConfig(c *Config) error {
	mutex.Lock()
	defer mutex.Unlock()
	ic, err := c.InjectorConfig()
	if err != nil {
		return err
	}
	injectorConfig = ic
	return nil
}
func GetInjectorConfig() *InjectorConfig {
	mutex.Lock()
	defer mutex.Unlock()
	return injectorConfig
}

type ContainerLogConfig struct {
	ContainerName string
	VolumeName    string
	LogPath       string
}

type LogsidecarConfig struct {
	ContainerLogConfigs ContainerLogConfigs `json:"containerLogConfigs,omitempty"`
}
type ContainerLogConfigs map[string]VolumeLogConfig // key: containerName; value: VolumeLogConfig
type VolumeLogConfig map[string][]string            // key: volumeName; value: logRelativePaths

func decodeLogsidecarConfig(confStr string) (*LogsidecarConfig, error) {
	confStr = strings.TrimSpace(confStr)
	if confStr != "" {
		conf := &LogsidecarConfig{}
		err := json.Unmarshal([]byte(confStr), conf)
		if err != nil {
			conf = nil
		}
		return conf, err
	}
	return nil, nil
}
