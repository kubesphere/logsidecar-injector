#!/bin/bash

set -e

certsdir="config/certs"

if [ "$(uname)" == "Darwin" ]; then
    sed -i "" -e "s!caBundle:.*!caBundle: $(cat ./config/certs/ca.crt |base64 )!" config/admission.yaml
    sed -i "" -e "s!server.key:.*!server.key: $(cat ./config/certs/server.key |base64 )!" config/admission.yaml
    sed -i "" -e "s!server.crt:.*!server.crt: $(cat ./config/certs/server.crt |base64 )!" config/admission.yaml
else
    sed -i -e "s!caBundle:.*!caBundle: $(cat ./config/certs/ca.crt |base64 -w 0)!" config/admission.yaml
    sed -i -e "s!server.key:.*!server.key: $(cat ./config/certs/server.key |base64 -w 0)!" config/admission.yaml
    sed -i -e "s!server.crt:.*!server.crt: $(cat ./config/certs/server.crt |base64 -w 0)!" config/admission.yaml
fi