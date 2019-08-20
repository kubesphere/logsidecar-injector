package injector

import (
	"encoding/json"
	"fmt"
	"github.com/mattbaird/jsonpatch"
	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog"
	"strings"
)

const (
	lscAnnotationName = "logging.kubesphere.io/logsidecar-config"
	lscContainerName  = "logsidecar-container-logging-kubesphere-io"
	lscImage          = "elastic/filebeat:6.7.0"
	lscVolumeName     = "logsidecar-config-volume-logging-kubesphere-io"

	lscInitContainerName = "logsidecar-init-container-logging-kubesphere-io"
	lscInitImage         = "alpine"
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
		klog.Error(err)
		return toAdmissionResponse(err)
	}
	reviewResponse := v1beta1.AdmissionResponse{}
	reviewResponse.Allowed = true
	podNN := pod.Namespace + ":" + pod.Name
	podSpec := &pod.Spec

	removeLogsidecarSpec(podSpec)

	if lscConfigStr, exists := pod.Annotations[lscAnnotationName]; exists {
		if lscConfigStr = strings.TrimSpace(lscConfigStr); lscConfigStr != "" {
			lscConfig, err := decodeLSCConfig(lscConfigStr)
			if err != nil {
				err = fmt.Errorf("unable to decode annotations[%s] in pod %s: %v", lscAnnotationName, podNN, err)
				klog.Error(err)
				return toAdmissionResponse(err)
			}
			logVolMounts, logAbsPaths := ParseLSCConf(lscConfig)
			vNames := make(map[string]struct{})
			for _, v := range podSpec.Volumes {
				vNames[v.Name] = struct{}{}
			}
			for _, vm := range logVolMounts {
				if _, exists := vNames[vm.Name]; !exists {
					err = fmt.Errorf("unable to find volume(%s) in pod %s", vm.Name, podNN)
					klog.Error(err)
					return toAdmissionResponse(err)
				}
			}
			addLogsidecarSpec(podSpec, logVolMounts, logAbsPaths)
		}
	}

	patch, err := createLogsidecarPatch(raw, &pod)
	if err != nil {
		err = fmt.Errorf("failed to create patch when inject logsidecar into pod %s: %v", podNN, err)
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

func removeLogsidecarSpec(podSpec *corev1.PodSpec) {
	for i, ic := range podSpec.InitContainers {
		if ic.Name == lscInitContainerName {
			podSpec.InitContainers = append(podSpec.InitContainers[:i], podSpec.InitContainers[i+1:]...)
		}
	}
	for i, ic := range podSpec.Containers {
		if ic.Name == lscContainerName {
			podSpec.Containers = append(podSpec.Containers[:i], podSpec.Containers[i+1:]...)
		}
	}
	for i, ic := range podSpec.Volumes {
		if ic.Name == lscVolumeName {
			podSpec.Volumes = append(podSpec.Volumes[:i], podSpec.Volumes[i+1:]...)
		}
	}
}

func addLogsidecarSpec(podSpec *corev1.PodSpec, logVolMounts []corev1.VolumeMount, logAbsPaths []string) {
	confVol := corev1.Volume{
		Name: lscVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	podSpec.Volumes = append(podSpec.Volumes, confVol)

	confVolMount := corev1.VolumeMount{
		Name:      lscVolumeName,
		MountPath: FilebeatConfDir,
	}

	initC := corev1.Container{
		Name:            lscInitContainerName,
		Image:           lscInitImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/sh"},
		Args:            []string{"-c", FilebeatConfigInitCMD(logAbsPaths)},
		VolumeMounts:    []corev1.VolumeMount{confVolMount},
	}
	podSpec.InitContainers = append(podSpec.InitContainers, initC)

	sidecarC := corev1.Container{
		Name:            lscContainerName,
		Image:           lscImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    lscCpuLimit,
				corev1.ResourceMemory: lscMemoryLimit,
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    lscCpuRequest,
				corev1.ResourceMemory: lscMemoryRequest,
			},
		},
		Args:         []string{"--path.config", FilebeatConfDir},
		VolumeMounts: append(logVolMounts, confVolMount),
	}
	podSpec.Containers = append(podSpec.Containers, sidecarC)
}

func ParseLSCConf(conf *LSCConfig) (logVolMounts []corev1.VolumeMount, logAbsPaths []string) {
	for cName, volumeLogConfig := range conf.ContainerLogConfigs {
		if cName = strings.TrimSpace(cName); cName == "" {
			continue
		}
		volLogRelPaths := make(map[string]map[string]struct{})
		for vName, logRelPaths := range volumeLogConfig {
			if vName = strings.TrimSpace(vName); vName == "" {
				continue
			}
			for _, logRelPath := range logRelPaths {
				if logRelPath = strings.TrimSpace(logRelPath); logRelPath == "" {
					continue
				}
				if volLogRelPaths[vName] == nil {
					volLogRelPaths[vName] = make(map[string]struct{})
				}
				volLogRelPaths[vName][logRelPath] = struct{}{}
			}
		}
		for vName, logRelPathSet := range volLogRelPaths {
			logVolMounts = append(logVolMounts, corev1.VolumeMount{
				Name:      vName,
				MountPath: "/" + vName,
			})
			for rPath, _ := range logRelPathSet {
				if rPath = strings.TrimSpace(rPath); rPath == "" {
					continue
				}
				logAbsPaths = append(logAbsPaths, "/"+vName+"/"+rPath)
			}
		}
	}
	return
}
