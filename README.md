# Logsidecar-injector

Logsidecar-injector is a Kubernetes mutating webhook server that adds a sidecar to your pod. This sidecar is just to forward logs from files on volumes to stdout.

# Install

To quickly install the logsidecar injector inside a cluster, just run the following command:

```bash
kubectl apply -f https://raw.githubusercontent.com/kubesphere/logsidecar-injector/master/config/bundle.yaml
```
> Note: it is default to install into the namespace `kubesphere-logging-system`

# Usage
Suppose you want to inject the log sidecar into your workloads in the namespace `default`:

- Firstly add a label to the namespace:
  ```bash
  kubectl label ns default logging.kubesphere.io/logsidecar-injection=enabled
  ```

- Then add an annotation to pod template of your workload:
  ```yaml
  spec:
    template:
      metadata:
        annotations:
          logging.kubesphere.io/logsidecar-config: 'VALUE'
  ```
    > Note: the `VALUE` is a json string which has format as follows:
    > ```json
    > {
    >     "containerLogConfigs": {
    >         "$containername": {
    >             "$volumename": [
    >                 "$relativepath"
    >             ]
    >         }
    >     }
    > }
    > ```
    > `relativepath` is relative path to mountpath within container `containername` at which volume `volumename` was mounted.

- Optionally customize your configuration of filebeat in sidecar container:  
Add `logging.kubesphere.io/logsidecar-filebeat-config-jsonpatch` annotation to pod template of your workload. The value of this annotation is a [jsonpatch](http://jsonpatch.com/) string. Logsidecar-injector will generate a new configuration based on default filebeat configuration and this patch, then apply it to the injected sidecar container for your specified workload pod. Here is an example:
  ```yaml
  spec:
    template:
      metadata:
        annotations:
          logging.kubesphere.io/logsidecar-filebeat-config-jsonpatch: '[{"op":"replace","path":"/filebeat.inputs/0/tail_file","value":true}]'
  ```