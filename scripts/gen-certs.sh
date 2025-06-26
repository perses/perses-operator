#!/bin/bash

set -e

CERT_DIR="/tmp/k8s-webhook-server/serving-certs"

mkdir -p "${CERT_DIR}"

openssl genrsa -out "${CERT_DIR}/ca.key" 2048

openssl req -x509 -new -nodes -key "${CERT_DIR}/ca.key" -sha256 -days 365 \
  -subj "/CN=Webhook-CA" -out "${CERT_DIR}/ca.crt"

openssl genrsa -out "${CERT_DIR}/tls.key" 2048

cat <<EOF > "${CERT_DIR}/server.conf"
[req]
prompt = no
distinguished_name = req_distinguished_name
req_extensions = v3_req

[req_distinguished_name]
CN = Webhook-Server

[v3_req]
keyUsage = critical, digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = perses-operator-webhook-service.perses-operator-system.svc
EOF

openssl req -new -key "${CERT_DIR}/tls.key" -out "${CERT_DIR}/tls.csr" -config "${CERT_DIR}/server.conf" -sha256

openssl x509 -req -in "${CERT_DIR}/tls.csr" -CA "${CERT_DIR}/ca.crt" -CAkey "${CERT_DIR}/ca.key" \
  -CAcreateserial -out "${CERT_DIR}/tls.crt" -days 365 -sha256 \
  -extfile "${CERT_DIR}/server.conf" -extensions v3_req


echo "Updating webhook manifests"

CA_BUNDLE=$(base64 -w 0 -i "${CERT_DIR}/ca.crt")
export CA_BUNDLE

WEBHOOK_PERSES="patches/webhook_in_perses.yaml"
WEBHOOK_PERSESDASHBOARDS="patches/webhook_in_persesdashboards.yaml"
WEBHOOK_PERSESDATASOURCES="patches/webhook_in_persesdatasource.yaml"
WEBHOOK_PERSESGLOBALDATASOURCES="patches/webhook_in_persesglobaldatasource.yaml"

yq e '.spec.conversion.webhook.clientConfig.caBundle = env(CA_BUNDLE)' \
  "config/crd/${WEBHOOK_PERSES}" > "config/local/${WEBHOOK_PERSES}"
yq e '.spec.conversion.webhook.clientConfig.caBundle = env(CA_BUNDLE)' \
  "config/crd/${WEBHOOK_PERSESDASHBOARDS}" > "config/local/${WEBHOOK_PERSESDASHBOARDS}"
yq e '.spec.conversion.webhook.clientConfig.caBundle =  env(CA_BUNDLE)' \
  "config/crd/${WEBHOOK_PERSESDATASOURCES}" > "config/local/${WEBHOOK_PERSESDATASOURCES}"
yq e '.spec.conversion.webhook.clientConfig.caBundle =  env(CA_BUNDLE)' \
  "config/crd/${WEBHOOK_PERSESGLOBALDATASOURCES}" > "config/local/${WEBHOOK_PERSESGLOBALDATASOURCES}"
