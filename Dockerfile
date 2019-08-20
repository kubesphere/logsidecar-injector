# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:latest
FROM alpine
WORKDIR /
COPY bin/injector injector
CMD ["/injector", \
    "-tls-cert-file", "/etc/logsidecar-injector/serve.crt",\
    "-tls-private-key-file", "/etc/logsidecar-injector/serve.key",\
    "-logtostderr", \
    "-v", "2"]
