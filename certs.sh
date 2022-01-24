#!/bin/bash

function stdGenerateCertificates() {

  local SERVICE="$1"
  local NAMESPACE="$2"
  local KEY_FILE="$3"
  local CRT_FILE="$4"

  if [[ "$SERVICE" == "" ]] || [[ "$NAMESPACE" == "" ]]; then
    echo "Namespace/Service is not specified. Generation skipped."
    return
  fi

  if [[ "$KEY_FILE" == "" ]] || [[ "$CRT_FILE" == "" ]]; then
    echo "Key/Crt file name is not specified. Generation skipped."
    return
  fi

  tmpdir=$(mktemp -d)

  cat <<EOF >>"${tmpdir}/csr.conf"
  [req]
  req_extensions = v3_req
  distinguished_name = req_distinguished_name
  [req_distinguished_name]
  [ v3_req ]
  basicConstraints = CA:FALSE
  keyUsage = nonRepudiation, digitalSignature, keyEncipherment
  extendedKeyUsage = serverAuth
  subjectAltName = @alt_names
  [alt_names]
  DNS.1 = ${SERVICE}
  DNS.2 = ${SERVICE}.${NAMESPACE}
  DNS.3 = ${SERVICE}.${NAMESPACE}.svc
EOF

  local __out=""

  openssl genrsa -out "${KEY_FILE}" 2048 >__openSsl.out 2>&1
  __out=$(cat __openSsl.out)
  if [[ ! "$?" -eq 0 ]]; then
    echo "$__out"
    return 1
  else
    echo "'openssl genrsa' output:\n$__out"
  fi

  openssl req -new -key "${KEY_FILE}" -subj "/CN=${SERVICE}.${NAMESPACE}.svc" -out "${tmpdir}/${SERVICE}.csr" -config "${tmpdir}/csr.conf" >__openSsl.out 2>&1
  __out=$(cat __openSsl.out)
  if [[ ! "$?" -eq 0 ]]; then
    echo "$__out"
    return 1
  else
    echo "'openssl req -new -key' output:\n$__out"
  fi

  openssl x509 -signkey "${KEY_FILE}" -in "${tmpdir}/${SERVICE}.csr" -req -days 365 -out "${CRT_FILE}" >__openSsl.out 2>&1
  __out=$(cat __openSsl.out)
  if [[ ! "$?" -eq 0 ]]; then
    echo "$__out"
    return 1
  else
    echo "'openssl x509 -signkey' output:\n$__out"
  fi

  rm -rf __openSsl.out
}

stdGenerateCertificates "events" "sre" "events.key" "events.crt"

echo "Crt..."
cat "events.crt" | base64 | tr -d '\n'
echo ""
echo "Key..."
cat "events.key" | base64 | tr -d '\n'
echo ""
echo "Bundle..."
cat "events.crt" | base64 | tr -d '\n'
echo ""