IMG ?= log-sidecar-injector:1.0
SERVICE_NAME ?= logsidecar-injector
NAMESPACE ?= kubesphere-logging-system

all: injector

# Build injector binary
injector: fmt vet
	CGO_ENABLED=0 go build -o bin/injector main.go

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Build the docker image
docker-build: injector
	docker build . -t ${IMG}



CERTSDIR ?= config/certs
ca.key:
	openssl genrsa -out ${CERTSDIR}/ca.key 2048

ca.crt: ca.key
	openssl req -x509 -new -nodes -key ${CERTSDIR}/ca.key \
		-subj "/C=CN/ST=HB/O=QC/CN=${SERVICE_NAME}-ca" \
		-sha256 -days 10000 -out ${CERTSDIR}/ca.crt

serve.key: ca.crt
	openssl genrsa -out ${CERTSDIR}/serve.key 2048

serve.crt: serve.key
	openssl req -new -sha256 \
		-key ${CERTSDIR}/serve.key \
		-subj "/C=CN/ST=HB/O=QC/CN=${SERVICE_NAME}.${NAMESPACE}.svc" \
		-out ${CERTSDIR}/serve.csr
	openssl x509 -req -in ${CERTSDIR}/serve.csr -CA ${CERTSDIR}/ca.crt \
		-CAkey ${CERTSDIR}/ca.key -CAcreateserial \
		-out ${CERTSDIR}/serve.crt -days 10000 -sha256

.PHONY: filebeat
filebeat:
	sed 's/^/    /g' config/filebeat.yml.template | sed '$$a\\n' > config/filebeat.yml.tmp
	sed 's/^/    /g' config/inputs.yml.template | sed '$$a\\n' > config/inputs.yml.tmp
	cat deploy/configmap.yaml.template | sed '/filebeat.yml.template/r config/filebeat.yml.tmp' \
        	| sed '/inputs.yml.template/r config/inputs.yml.tmp' > deploy/configmap.yaml

.PHONY: secret
secret: serve.crt
	cat deploy/secret.yaml.template | sed 's/<<SERVEKEY>>/$(shell cat ${CERTSDIR}/serve.key | base64 -w 0)/g' \
		| sed 's/<<SERVECRT>>/$(shell cat ${CERTSDIR}/serve.crt | base64 -w 0)/g' > deploy/secret.yaml

.PHONY: webhook
webhook: secret
	cat deploy/webhook.yaml.template | sed 's/<<CACERT>>/$(shell cat ${CERTSDIR}/ca.crt |base64 -w 0)/g' > deploy/webhook.yaml