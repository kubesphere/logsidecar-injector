package injector

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"
)

func TestSidecarConfig(t *testing.T) {
	tempDir, err := ioutil.TempDir("", t.Name())
	if err != nil {
		t.Fatalf("TempDir %s: %v", t.Name(), err)
	}
	defer os.RemoveAll(tempDir)

	var config = SidecarConfig{
		Container: ContainerConfig{
			Image:           SidecarContainerDefaultImage,
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("100Mi"),
				},
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		},
		InitContainer: ContainerConfig{
			Image:           SidecarInitContainerDefaultImage,
			ImagePullPolicy: v1.PullIfNotPresent,
			Resources: v1.ResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("100Mi"),
				},
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("100m"),
					v1.ResourceMemory: resource.MustParse("100Mi"),
				},
			},
		},
	}
	scBytes, err := yaml.Marshal(&config)
	if err != nil {
		log.Fatal("marshal sidecar config: ", err)
	}
	ioutil.WriteFile(filepath.Join(tempDir, "sidecar.yaml"), scBytes, 0644)

	gotSidecarConfig, err := sidecarConfig(filepath.Join(tempDir, "sidecar.yaml"))
	if err != nil {
		t.Fatal("parse sidecar config: ", err)
	}

	if diff := cmp.Diff(&config, gotSidecarConfig); diff != "" {
		t.Fatal(diff)
	}

}
