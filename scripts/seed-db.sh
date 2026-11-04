#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_NAME="${DB_CONTAINER_NAME:-adelson-postgres}"
DB_USER="${POSTGRES_USER:-user}"
DB_NAME="${POSTGRES_DB:-adelson}"

if command -v podman >/dev/null 2>&1; then
	RUNTIME="podman"
elif command -v docker >/dev/null 2>&1; then
	RUNTIME="docker"
else
	echo "Error: install Podman or Docker before running this script." >&2
	exit 1
fi

if ! "${RUNTIME}" container inspect "${CONTAINER_NAME}" >/dev/null 2>&1; then
	echo "Error: container ${CONTAINER_NAME} does not exist. Run ./scripts/setup-db.sh first." >&2
	exit 1
fi

running="$("${RUNTIME}" container inspect --format '{{.State.Running}}' "${CONTAINER_NAME}")"
if [[ "${running}" != "true" ]]; then
	echo "Starting container ${CONTAINER_NAME}..."
	"${RUNTIME}" start "${CONTAINER_NAME}" >/dev/null
fi

echo "Applying database/seed.sql to ${DB_NAME} (idempotent; safe to re-run)..."
"${RUNTIME}" exec -i "${CONTAINER_NAME}" \
	psql -U "${DB_USER}" -d "${DB_NAME}" -v ON_ERROR_STOP=1 \
	< "${ROOT_DIR}/database/seed.sql"

echo "Seed complete. Test login:"
echo "  curl -s -X POST http://localhost:8080/api/v1/auth/login \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"correoElectronico\":\"admin@adelson.example\",\"contrasena\":\"Admin12345\"}'"
