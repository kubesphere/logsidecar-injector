package injector

import (
	"crypto/tls"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"k8s.io/api/core/v1"
	"sync"
	"text/template"
)

const (
	SidecarContainerDefaultImage     = "elastic/filebeat:6.7.0"
	SidecarInitContainerDefaultImage = "alpine:3.9"
)

type Config struct {
	CertFile string
	KeyFile  string

	FilebeatConfigFile string
	SidecarConfigFile  string
}

type ContainerConfig struct {
	Image           string                  `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullPolicy v1.PullPolicy           `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Resources       v1.ResourceRequirements `json:"resources" yaml:"resources"`
}

type SidecarConfig struct {
	InitContainer ContainerConfig `json:"initContainer" yaml:"initContainer"`
	Container     ContainerConfig `json:"container" yaml:"container"`
}

type InjectorConfig struct {
	SidecarConfig          SidecarConfig
	FilebeatConfigTemplate *template.Template
}

func (c *Config) AddFlags() {
	flag.StringVar(&c.CertFile, "tls-cert-file", "/etc/logsidecar-injector/certs/server.crt",
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	flag.StringVar(&c.KeyFile, "tls-private-key-file", "/etc/logsidecar-injector/certs/server.key",
		"File containing the default x509 private key matching --tls-cert-file.")

	flag.StringVar(&c.SidecarConfigFile, "sidecar-config-file", "/etc/logsidecar-injector/config/sidecar.yaml",
		"File containing config of injected containers etc.")
	flag.StringVar(&c.FilebeatConfigFile, "filebeat-config-file", "/etc/logsidecar-injector/config/filebeat.yaml",
		"File containing filebeat config")
}

func (c *Config) TLSConfig(stop <-chan struct{}, reloadCh <-chan chan error) (*tls.Config, error) {
	sCert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
	if err != nil {
		return nil, err
	}
	var m sync.Mutex
	go func() {
		for {
			select {
			case errc := <-reloadCh:
				cert, e := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
				errc <- e
				if e == nil {
					func() {
						m.Lock()
						defer m.Unlock()
						sCert = cert
					}()
				}
			case <-stop:
				return
			}
		}
	}()
	return &tls.Config{
		GetCertificate: func(_ *tls.ClientHelloInfo) (cert *tls.Certificate, e error) {
			m.Lock()
			defer m.Unlock()
			return &sCert, nil
		},
	}, nil
}

func (c *Config) InjectorConfig() (*InjectorConfig, error) {
	ic := &InjectorConfig{}
	scontent, err := ioutil.ReadFile(c.SidecarConfigFile)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(scontent, &ic.SidecarConfig); err != nil {
		return nil, err
	}
	tmpl, err := template.ParseFiles(c.FilebeatConfigFile)
	if err != nil {
		return nil, fmt.Errorf("error to parse %s to tempalte: %v", c.FilebeatConfigFile, err)
	}
	ic.FilebeatConfigTemplate = tmpl
	if ic.SidecarConfig.Container.Image == "" {
		ic.SidecarConfig.Container.Image = SidecarContainerDefaultImage
	}
	if ic.SidecarConfig.InitContainer.Image == "" {
		ic.SidecarConfig.InitContainer.Image = SidecarInitContainerDefaultImage
	}
	return ic, nil
}
