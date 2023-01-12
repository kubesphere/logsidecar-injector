REPO ?= kubesphere
TAG ?= $(shell cat VERSION | tr -d " \t\n\r")
IMAGE = $(REPO)/log-sidecar-injector:$(TAG)

SERVICE_NAME ?= logsidecar-injector-admission
NAMESPACE ?= kubesphere-logging-system

ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Build the docker image
docker-build:
	docker build -t $(IMAGE) .

docker-cross-build:
	docker buildx build --push --platform linux/amd64,linux/arm64 -t $(IMAGE) .

# Push the docker image
docker-push:
	docker push $(IMAGE)

deploy: generate
	kubectl apply -f config/bundle.yaml

ca-secret:
	./hack/certs.sh --service $(SERVICE_NAME) --namespace $(NAMESPACE)

update-cert: ca-secret
	./hack/update-cert.sh

generate:
	cd config && $(GOBIN)/kustomize edit set image injector=$(IMAGE)
	$(GOBIN)/kustomize build config > config/bundle.yaml