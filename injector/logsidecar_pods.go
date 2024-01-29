package injector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mattbaird/jsonpatch"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"path/filepath"
	"strings"
)

const (
	logsidecarAnnotationName              = "logging.kubesphere.io/logsidecar-config"
	logsidecarFilebeatPatchAnnotationName = "logging.kubesphere.io/logsidecar-filebeat-config-jsonpatch"
	logsidecarVectorPatchAnnotationName   = "logging.kubesphere.io/logsidecar-vector-config-jsonpatch"
	logsidecarInitContainerName           = "logsidecar-init-container-logging-kubesphere-io"
	logsidecarContainerName               = "logsidecar-container-logging-kubesphere-io"
	logsidecarVolumeName                  = "logsidecar-config-volume-logging-kubesphere-io"
)

func MutateLogsidecarPods(ar v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	klog.V(2).Info("inject logsidecar into pods")
	podResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	if ar.Request.Resource != podResource {
		err := fmt.Errorf("expect resource to be %s", podResource)
		klog.Error(err)
		return toAdmissionResponse(err)
	}

	raw := ar.Request.Object.Raw
	pod := corev1.Pod{}
	deserializer := codecs.UniversalDeserializer()
	if _, _, err := deserializer.Decode(raw, nil, &pod); err != nil {
		err = fmt.Errorf("fail to decode admission request: %v", err)
		klog.Error(err)
		return toAdmissionResponse(err)
	}
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true
	podNN := pod.Namespace + ":" + pod.Name
	podSpec := &pod.Spec

	removeLogsidecarPart(podSpec)

	if confStr, exists := pod.Annotations[logsidecarAnnotationName]; exists {
		if confStr = strings.TrimSpace(confStr); confStr != "" {
			lscConfig, err := decodeLogsidecarConfig(confStr)
			if err != nil {
				err = fmt.Errorf("unable to decode annotations[%s] in pod %s: %v",
					logsidecarAnnotationName, podNN, err)
				klog.Error(err)
				return toAdmissionResponse(err)
			}

			if err = addLogsidecarPart(&pod, lscConfig); err != nil {
				err = fmt.Errorf("faild to inject logsidecar into pod %s: %v", podNN, err)
				klog.Error(err)
				return toAdmissionResponse(err)
			}
		}
	}

	patch, err := createLogsidecarPatch(raw, &pod)
	if err != nil {
		err = fmt.Errorf("failed to create patch of pod %s: %v", podNN, err)
		klog.Error(err)
		return toAdmissionResponse(err)
	}
	if patch != nil {
		reviewResponse.Patch = patch
		patchType := v1beta1.PatchTypeJSONPatch
		reviewResponse.PatchType = &patchType
	}

	return &reviewResponse
}

func createLogsidecarPatch(raw []byte, mutated runtime.Object) ([]byte, error) {
	mu, err := json.Marshal(mutated)
	if err != nil {
		return nil, err
	}
	patch, err := jsonpatch.CreatePatch(raw, mu)
	if err != nil {
		return nil, err
	}
	if len(patch) > 0 {
		return json.Marshal(patch)
	}
	return nil, nil
}

func removeLogsidecarPart(podSpec *corev1.PodSpec) {
	for i, c := range podSpec.InitContainers {
		if c.Name == logsidecarInitContainerName {
			podSpec.InitContainers = append(podSpec.InitContainers[:i], podSpec.InitContainers[i+1:]...)
		}
	}
	for i, c := range podSpec.Containers {
		if c.Name == logsidecarContainerName {
			podSpec.Containers = append(podSpec.Containers[:i], podSpec.Containers[i+1:]...)
		}
	}
	for i, v := range podSpec.Volumes {
		if v.Name == logsidecarVolumeName {
			podSpec.Volumes = append(podSpec.Volumes[:i], podSpec.Volumes[i+1:]...)
		}
	}
}

const (
	logsidecarConfigDir    = "/etc/logsidecar"
	filebeatConfigFileName = "filebeat.yaml"
	vectorConfigFileName   = "vector.yaml"
)

func addLogsidecarPart(pod *corev1.Pod, conf *LogsidecarConfig) error {
	cvmMap := make(map[string]map[string]string) // containerName: volumeName: mountPath
	for _, c := range pod.Spec.Containers {
		if len(c.VolumeMounts) == 0 {
			continue
		}
		vmMap := make(map[string]string) // volumeName: mountPath
		for _, vm := range c.VolumeMounts {
			vmMap[vm.Name] = vm.MountPath
		}
		cvmMap[c.Name] = vmMap
	}
	var volumeMounts []corev1.VolumeMount
	var filebeatLogPaths []string
	for containerName, vpMap := range conf.ContainerLogConfigs {
		for volumeName, logRelativePaths := range vpMap {
			if len(logRelativePaths) == 0 {
				continue
			}
			if volumeMountMap, ok := cvmMap[containerName]; ok {
				if mountPath, ok := volumeMountMap[volumeName]; ok {
					mountPath := filepath.Clean(fmt.Sprintf("/container-%s/%s", containerName, mountPath))
					volumeMounts = append(volumeMounts, corev1.VolumeMount{
						Name: volumeName, MountPath: mountPath})
					for _, relativePath := range logRelativePaths {
						if relativePath = strings.TrimSpace(relativePath); relativePath != "" {
							filebeatLogPaths = append(filebeatLogPaths,
								filepath.Clean(fmt.Sprintf("%s/%s", mountPath, relativePath)))
						}
					}
				}
			}
		}
	}

	if len(filebeatLogPaths) == 0 {
		return nil
	}

	iconfig := GetInjectorConfig()
	tmpl := iconfig.VectorConfigTemplate
	jsonPatch, _ := pod.Annotations[logsidecarVectorPatchAnnotationName]
	configFile := vectorConfigFileName
	if iconfig.SidecarType == SidecarTypeFilebeat {
		tmpl = iconfig.FilebeatConfigTemplate
		jsonPatch, _ = pod.Annotations[logsidecarFilebeatPatchAnnotationName]
		configFile = filebeatConfigFileName
	}

	// echo command writes filebeat config to volume shared by filebeat container
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, struct {
		Paths []string
	}{filebeatLogPaths}); err != nil {
		return err
	}
	configYaml := buffer.String()
	if jsonPatch = strings.TrimSpace(jsonPatch); jsonPatch != "" {
		newYaml, err := PatchYaml(configYaml, jsonPatch)
		if err != nil {
			return err
		}
		configYaml = newYaml
	}
	configEcho := JoinLines(configYaml, "echo \"",
		fmt.Sprintf("\" >> %s/%s ; ", logsidecarConfigDir, configFile))

	logsidecarVolume := corev1.Volume{
		Name:         logsidecarVolumeName,
		VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
	}
	logsidecarVolumeMount := corev1.VolumeMount{
		Name:      logsidecarVolumeName,
		MountPath: logsidecarConfigDir,
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, logsidecarVolume)
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, corev1.Container{
		Name:            logsidecarInitContainerName,
		Image:           iconfig.SidecarConfig.InitContainer.Image,
		ImagePullPolicy: iconfig.SidecarConfig.InitContainer.ImagePullPolicy,
		Resources:       iconfig.SidecarConfig.InitContainer.Resources,
		Command:         []string{"/bin/sh"},
		Args:            []string{"-c", configEcho},
		VolumeMounts:    []corev1.VolumeMount{logsidecarVolumeMount},
	})
	image := iconfig.SidecarConfig.VectorContainer.Image
	imagePullPolicy := iconfig.SidecarConfig.VectorContainer.ImagePullPolicy
	resources := iconfig.SidecarConfig.VectorContainer.Resources
	if iconfig.SidecarType == SidecarTypeFilebeat {
		image = iconfig.SidecarConfig.FilebeatContainer.Image
		imagePullPolicy = iconfig.SidecarConfig.FilebeatContainer.ImagePullPolicy
		resources = iconfig.SidecarConfig.FilebeatContainer.Resources
	}
	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:            logsidecarContainerName,
		Image:           image,
		ImagePullPolicy: imagePullPolicy,
		Resources:       resources,
		Args:            []string{"-c", fmt.Sprintf("%s/%s", logsidecarConfigDir, configFile)},
		VolumeMounts:    append(volumeMounts, logsidecarVolumeMount),
	})
	return nil
}
