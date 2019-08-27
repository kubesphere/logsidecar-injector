# Logsidecar injector

A `MutatingAdmissionWebhook` that adds a sidecar to your pod. This sidecar is just for forwarding file log on the volume.

## Design
Automatic sidecar injection adds the sidecar logger into user-created pods. It uses a MutatingWebhook to append the sidecar’s containers to each pod’s template spec during creation time. Injection can be scoped to particular sets of namespaces using the webhooks namespaceSelector mechanism. How to do the injection depends on the specified annotation in the pod template's metadata.

## Install And Deploy
1. `make docker-build` builds the image
1. `make .PHONY` generates the yaml files
1. just deploy all by follows:
    ```bash
    kubectl create -f deploy/configmap.yaml
    kubectl create -f deploy/secret.yaml
    kubectl create -f deploy/logsidecar-injector.yaml
    kubectl create -f deploy/webhook.yaml
    ```

## Usage
You can use it in your workload by follows:
1. add a `logging.kubesphere.io/logsidecar-injection` label for your namespace, which's value can be `enabled` or `disabled`
    ```yml
    apiVersion: v1
    kind: Namespace
    metadata:
    name: default
    labels:
        logging.kubesphere.io/logsidecar-injection: enabled
    ```
2. add a `logging.kubesphere.io/logsidecar-config` annotation in the pod template spec's metadata  
    ```yml
    spec:
      template:
        metadata:
        annotations:
            logging.kubesphere.io/logsidecar-config: 'value'
    ```
    the value format of this annotation should be as follows (there is no tab or enter actually)
    ```json
    {
        "containerLogConfigs": {
            "$containername1": {
                "$volumename1": [
                    "log/*.log"
                ]
            },
            "$containername2": {
                "$volumename2": [
                    "log/start*.log",
                    "log/sublog/**/*.log"
                ]
            }
        }
    }
    ```