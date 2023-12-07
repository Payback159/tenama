#!/usr/bin/env bash

set -e

PREFIX=tenama
INFIX=infix
SUFFIX=suffix
DURATION=1m
USERS=mustermann
NSN=${PREFIX}-${INFIX}-${SUFFIX}

echo "Create namespace"

curl -X 'POST' \
  'http://localhost:8080/namespace' \
  -H 'accept: application/yaml' \
  -H 'Content-Type: application/json' \
  -d '{
  "duration": "'${DURATION}'",
  "infix": "'${INFIX}'",
  "suffix": "'${SUFFIX}'",
  "users": "'${USERS}'"
}'

echo "Find created namespace"

curl -X 'GET' \
  'http://localhost:8080/namespace/'${NSN}'' \
  -H 'accept: application/yaml'

sleep 5
echo "Delete created namespace"

curl -X 'DELETE' \
  'http://localhost:8080/namespace/'${NSN}'' \
  -H 'accept: application/yaml' \
  -H 'Content-Type: application/json'
