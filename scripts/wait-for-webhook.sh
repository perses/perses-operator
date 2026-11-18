#!/bin/bash

# Waits until the operator's conversion webhook answers requests.
#
# A Deployment that reports Available does not guarantee that the webhook
# service already routes to a listening pod. A server-side dry run of a v1alpha1
# object goes through the conversion webhook without creating anything, so a
# successful dry run proves that the whole path works.
#
# Usage: wait-for-webhook.sh [timeout-seconds]

set -euo pipefail

TIMEOUT_SECONDS="${1:-120}"

probe() {
	kubectl apply --request-timeout=10s --dry-run=server -n default -f - <<EOF
apiVersion: perses.dev/v1alpha1
kind: PersesDatasource
metadata:
  name: webhook-readiness-probe
spec:
  config:
    display:
      name: "Webhook readiness probe"
    default: false
    plugin:
      kind: "PrometheusDatasource"
      spec:
        directUrl: "https://prometheus.demo.prometheus.io"
EOF
}

echo ">> Waiting for the conversion webhook to answer..."
deadline=$((SECONDS + TIMEOUT_SECONDS))
until output=$(probe 2>&1); do
	if ((SECONDS >= deadline)); then
		echo "conversion webhook did not answer within ${TIMEOUT_SECONDS}s, last error:" >&2
		echo "${output}" >&2
		exit 1
	fi
	sleep 1
done
echo ">> Conversion webhook is answering"
