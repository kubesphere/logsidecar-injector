package injector

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"path/filepath"
	"testing"
	"text/template"
)

func TestLogsidecarPodMutate(t *testing.T) {
	filebeatConfig := `
filebeat.inputs:
  - type: log
    enabled: true
    paths:
    {{range .Paths}}
    - {{.}}
    {{end}}
output.console:
  codec.format:
    string: '%{[message]}'
logging.level: warning
`
	tmpl := template.New("filebeat.yaml")
	_, err := tmpl.Parse(filebeatConfig)
	if err != nil {
		panic(err)
	}
	injectorConfig = &InjectorConfig{
		FilebeatConfigTemplate: tmpl,
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				logsidecarAnnotationName: "{\"containerLogConfigs\": {\"app-container\": {\"datavolume\": [\"log/*.log\"]}}}",
			},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name:         "datavolume",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			}},
			Containers: []corev1.Container{{
				Name:    "app-container",
				Image:   "alpine",
				Command: []string{"/bin/sh"},
				Args:    []string{"-c", "if [ ! -d /data/log ];then mkdir -p /data/log;fi; while true; do date >> /data/log/app-test.log; sleep 30;done"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "datavolume",
					MountPath: "/data",
				}},
			}},
		},
	}
	mutatedPod := pod.DeepCopy()
	lscConfig, err := decodeLogsidecarConfig(mutatedPod.Annotations[logsidecarAnnotationName])
	if err != nil {
		panic(err)
	}
	err = addLogsidecarPart(&mutatedPod.Spec, lscConfig, "")
	if err != nil {
		panic(err)
	}

	expectedPod := pod.DeepCopy()
	var buffer bytes.Buffer
	if err := injectorConfig.FilebeatConfigTemplate.Execute(&buffer, struct {
		Paths []string
	}{[]string{filepath.Clean("/container-app-container/data/log/*.log")}}); err != nil {
		panic(err)
	}
	fbConfigEcho := JoinLines(buffer.String(), "echo \"",
		fmt.Sprintf("\" >> %s/%s ; ", logsidecarConfigDir, filebeatConfigFileName))

	expectedPod.Spec.InitContainers = []corev1.Container{{
		Name:            logsidecarInitContainerName,
		Image:           injectorConfig.SidecarConfig.InitContainer.Image,
		ImagePullPolicy: injectorConfig.SidecarConfig.InitContainer.ImagePullPolicy,
		Resources:       injectorConfig.SidecarConfig.InitContainer.Resources,
		Command:         []string{"/bin/sh"},
		Args:            []string{"-c", fbConfigEcho},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      logsidecarVolumeName,
			MountPath: logsidecarConfigDir,
		}},
	}}
	expectedPod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:            logsidecarContainerName,
		Image:           injectorConfig.SidecarConfig.Container.Image,
		ImagePullPolicy: injectorConfig.SidecarConfig.Container.ImagePullPolicy,
		Resources:       injectorConfig.SidecarConfig.Container.Resources,
		Args:            []string{"-c", fmt.Sprintf("%s/%s", logsidecarConfigDir, filebeatConfigFileName)},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "datavolume",
			MountPath: filepath.Clean("/container-app-container/data"),
		}, {
			Name:      logsidecarVolumeName,
			MountPath: logsidecarConfigDir,
		}},
	})
	expectedPod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name:         logsidecarVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	})

	//bs1, err := yaml.Marshal(expectedPod)
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println(string(bs1))
	//fmt.Println("-------------")
	//bs2, err := yaml.Marshal(mutatedPod)
	//if err != nil {
	//	panic(err)
	//}
	//fmt.Println(string(bs2))

	assert.Equal(t, expectedPod, mutatedPod)
}
