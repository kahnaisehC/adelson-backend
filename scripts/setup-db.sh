#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_NAME="${DB_CONTAINER_NAME:-adelson-postgres}"
VOLUME_NAME="${DB_VOLUME_NAME:-adelson-postgres-data}"
DB_USER="${POSTGRES_USER:-user}"
DB_PASSWORD="${POSTGRES_PASSWORD:-password}"
DB_NAME="${POSTGRES_DB:-adelson}"
DB_PORT="${POSTGRES_PORT:-5432}"
IMAGE="${POSTGRES_IMAGE:-docker.io/library/postgres:16-alpine}"

if command -v podman >/dev/null 2>&1; then
	RUNTIME="podman"
elif command -v docker >/dev/null 2>&1; then
	RUNTIME="docker"
else
	echo "Error: install Podman or Docker before running this script." >&2
	exit 1
fi

echo "Using ${RUNTIME}"

"${RUNTIME}" volume create "${VOLUME_NAME}" >/dev/null

if "${RUNTIME}" container inspect "${CONTAINER_NAME}" >/dev/null 2>&1; then
	running="$("${RUNTIME}" container inspect --format '{{.State.Running}}' "${CONTAINER_NAME}")"
	if [[ "${running}" != "true" ]]; then
		echo "Starting existing container ${CONTAINER_NAME}..."
		"${RUNTIME}" start "${CONTAINER_NAME}" >/dev/null
	else
		echo "Container ${CONTAINER_NAME} is already running."
	fi
else
	echo "Creating PostgreSQL container ${CONTAINER_NAME}..."
	"${RUNTIME}" run --detach \
		--name "${CONTAINER_NAME}" \
		--restart unless-stopped \
		--env "POSTGRES_USER=${DB_USER}" \
		--env "POSTGRES_PASSWORD=${DB_PASSWORD}" \
		--env "POSTGRES_DB=${DB_NAME}" \
		--publish "${DB_PORT}:5432" \
		--volume "${VOLUME_NAME}:/var/lib/postgresql/data" \
		--volume "${ROOT_DIR}/database/setup.sql:/docker-entrypoint-initdb.d/001_setup.sql:ro" \
		"${IMAGE}" >/dev/null
fi

echo "Waiting for PostgreSQL..."
for _ in {1..30}; do
	if "${RUNTIME}" exec "${CONTAINER_NAME}" pg_isready -U "${DB_USER}" -d "${DB_NAME}" >/dev/null 2>&1; then
		echo "PostgreSQL is ready."
		echo "DATABASE_URL=postgres://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"
		exit 0
	fi
	sleep 1
done

echo "Error: PostgreSQL did not become ready in 30 seconds." >&2
"${RUNTIME}" logs "${CONTAINER_NAME}" >&2 || true
exit 1
