package injector

import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"sync"
	"text/template"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	SidecarTypeFilebeat                  = "filebeat"
	SidecarTypeVector                    = "vector"
	SidecarContainerDefaultFilebeatImage = "elastic/filebeat:6.7.0"
	SidecarContainerDefaultVectorImage   = "timberio/vector:0.34.1-distroless-static"
	SidecarInitContainerDefaultImage     = "alpine:3.9"
)

type Config struct {
	CertFile string
	KeyFile  string

	SidecarType string

	FilebeatConfigFile string
	SidecarConfigFile  string
	VectorConfigFile   string
}

type ContainerConfig struct {
	Image           string                  `json:"image,omitempty" yaml:"image,omitempty"`
	ImagePullPolicy v1.PullPolicy           `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	Resources       v1.ResourceRequirements `json:"resources" yaml:"resources"`
}

type SidecarConfig struct {
	InitContainer     ContainerConfig `json:"initContainer" yaml:"initContainer"`
	FilebeatContainer ContainerConfig `json:"filebeatContainer,omitempty" yaml:"filebeatContainer,omitempty"`
	VectorContainer   ContainerConfig `json:"vectorContainer,omitempty" yaml:"vectorContainer,omitempty"`
}

type InjectorConfig struct {
	SidecarType            string
	SidecarConfig          SidecarConfig
	FilebeatConfigTemplate *template.Template
	VectorConfigTemplate   *template.Template
}

func (c *Config) AddFlags() {
	flag.StringVar(&c.CertFile, "tls-cert-file", "/etc/logsidecar-injector/certs/server.crt",
		"File containing the default x509 Certificate for HTTPS. (CA cert, if any, concatenated after server cert).")
	flag.StringVar(&c.KeyFile, "tls-private-key-file", "/etc/logsidecar-injector/certs/server.key",
		"File containing the default x509 private key matching --tls-cert-file.")
	flag.StringVar(&c.SidecarType, "sidecar-type", SidecarTypeVector, "Type of sidecar to inject. Supported values: filebeat, vector")
	flag.StringVar(&c.SidecarConfigFile, "sidecar-config-file", "/etc/logsidecar-injector/config/sidecar.yaml",
		"File containing config of injected containers etc.")
	flag.StringVar(&c.FilebeatConfigFile, "filebeat-config-file", "/etc/logsidecar-injector/config/filebeat.yaml",
		"File containing filebeat config")
	flag.StringVar(&c.VectorConfigFile, "vector-config-file", "/etc/logsidecar-injector/config/vector.yaml",
		"File containing vector config")
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

func sidecarConfig(sidecarConfigFile string) (*SidecarConfig, error) {
	var sidecarConfig SidecarConfig
	scontent, err := ioutil.ReadFile(sidecarConfigFile)
	if err != nil {
		return nil, err
	}
	if err = yaml.Unmarshal(scontent, &sidecarConfig); err != nil {
		return nil, err
	}
	return &sidecarConfig, nil
}

func (c *Config) InjectorConfig() (*InjectorConfig, error) {
	ic := &InjectorConfig{
		SidecarType: c.SidecarType,
	}

	sc, err := sidecarConfig(c.SidecarConfigFile)
	if err != nil {
		return nil, err
	}
	ic.SidecarConfig = *sc

	if c.SidecarType == SidecarTypeVector {
		vectorTmpl, err := template.ParseFiles(c.VectorConfigFile)
		if err != nil {
			return nil, fmt.Errorf("error to parse %s to tempalte: %v", c.VectorConfigFile, err)
		}
		ic.VectorConfigTemplate = vectorTmpl
		if ic.SidecarConfig.VectorContainer.Image == "" {
			ic.SidecarConfig.VectorContainer.Image = SidecarContainerDefaultVectorImage
		}
	} else if c.SidecarType == SidecarTypeFilebeat {
		filebeatTmpl, err := template.ParseFiles(c.FilebeatConfigFile)
		if err != nil {
			return nil, fmt.Errorf("error to parse %s to tempalte: %v", c.FilebeatConfigFile, err)
		}
		ic.FilebeatConfigTemplate = filebeatTmpl
		if ic.SidecarConfig.FilebeatContainer.Image == "" {
			ic.SidecarConfig.FilebeatContainer.Image = SidecarContainerDefaultFilebeatImage
		}
	} else {
		return nil, fmt.Errorf("sidecar type %s not supported", c.SidecarType)
	}

	if ic.SidecarConfig.InitContainer.Image == "" {
		ic.SidecarConfig.InitContainer.Image = SidecarInitContainerDefaultImage
	}

	return ic, nil
}
