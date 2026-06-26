#!/usr/bin/env bash
set -euo pipefail

: "${OPENCOST_BROKER_URL:?OPENCOST_BROKER_URL is required}"
: "${OPENCOST_BROKER_TOKEN:?OPENCOST_BROKER_TOKEN is required}"

broker_url="${OPENCOST_BROKER_URL%/}"

echo "Checking broker liveness..."
curl -fsS "${broker_url}/healthz"
echo

echo "Checking broker pod list..."
curl -fsS \
  -H "Authorization: Bearer ${OPENCOST_BROKER_TOKEN}" \
  -H "Accept: application/json" \
  "${broker_url}/v1/pods"
echo

echo "Checking broker chaos scenario discovery..."
curl -fsS \
  -H "Authorization: Bearer ${OPENCOST_BROKER_TOKEN}" \
  -H "Accept: application/json" \
  "${broker_url}/v1/chaos"
echo

echo "ops-broker smoke checks passed"
