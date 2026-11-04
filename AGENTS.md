# AGENTS.md

Go REST backend for "Adelson" (real-estate catalog + CRM + AdelAI WhatsApp assistant).
Module is `adelson-backend`; the API main lives at `./cmd/api` (NOT repo root).

## Commands

```bash
go run ./cmd/api                 # start API (loads .env; default :8080)
go run ./cmd/create-admin -nombre Nombre -apellido Apellido   # bootstrap/update first admin (needs ADMIN_EMAIL/ADMIN_PASSWORD)
gofmt -w cmd internal            # format (only targets; run before committing)
go test ./...                    # tests need no database
go vet ./...
go test ./internal/httpapi -run TestHealth   # single test
```

Go 1.26 is required (`go.mod`); toolchain in use is 1.26.5.

## Layout

- `cmd/api/main.go` — wiring, timeouts, graceful shutdown.
- `cmd/create-admin/main.go` — creates/updates the first `Administrador`, reassigns demo properties/inquiries, deactivates the demo agent.
- `internal/config` — env loading (`godotenv`), config struct.
- `internal/httpapi/` — routing + handlers + middleware: `server.go` (routes/core), `auth.go` (login/refresh/logout/me), `admin.go` (admin panel).
- `internal/store/` — all SQL/data access (pgx via `database/sql`): `store.go` (core), `admin.go` (admin panel).
- `internal/storage/r2.go` — Cloudflare R2 presigned uploads/deletes.
- `database/setup.sql` — schema source of truth. `database/migrations/` — incremental changes for existing DBs. `docs/api/api_documentation.md` — API contract.

## Config / DB

- `config.Load()` reads `.env`; exported env vars win. `JWT_SECRET` is required. `COOKIE_SECURE` defaults to `true` (set `false` for local HTTP). `ADDR` or `PORT` selects listen addr.
- Local DB: `./scripts/setup-db.sh` (picks podman then docker, creates persistent volume).
  Gotcha: `database/setup.sql` is mounted into `docker-entrypoint-initdb.d`, so it runs **only on first volume creation**. After schema edits, apply manually: `psql "$DATABASE_URL" -f database/setup.sql`; for existing DBs apply `database/migrations/*.sql` in order.
- Dev seed data: `./scripts/seed-db.sh` applies `database/seed.sql`. It merges the public-site catalog (demo agent + 8 properties + images) with dev CRM users: `Administrador` `admin@adelson.example` / `Admin12345`, `Agente` `carlos@adelson.example` / `Agente12345`, 3 clients, 5 extra properties, preferences, inquiries and chat sessions. Passwords are real bcrypt hashes, so login works. It is **idempotent** (`ON CONFLICT DO NOTHING`); a commented `TRUNCATE` block at the top enables a clean reset.
- First real admin: `ADMIN_EMAIL=... ADMIN_PASSWORD=... go run ./cmd/create-admin -nombre ... -apellido ...`. Don't re-run `database/neon_production_sync.sql` after creating the real admin (it rotates the demo password).
- `store.Open("")` returns a usable empty store; handlers then return `DATABASE_UNAVAILABLE` (503) instead of panicking.

## API conventions (differs from defaults)

- JSON envelopes: success `{"data": ..., "meta": {...}}`, error `{"error": {"code","message","details","requestId"}}`. Use `writeData` / `writePaginated` / `writeError`, never raw `json.Encode`.
- Request/response fields are **Spanish camelCase** (`correoElectronico`, `contrasena`, `tipoOperacion`, `cantBanos`). Query filters too (`page`, `pageSize`, `incluirEliminados`).
- `decodeJSON` sets `DisallowUnknownFields`: any extra JSON key yields `400 INVALID_REQUEST`.
- IDs must be canonical 36-char UUIDs (`validID`); list endpoints paginate (`page`/`pageSize`, default 20, max 100).
- Routing uses Go 1.22 method+path `http.ServeMux` patterns, registered in `New()`. Add new routes there.
- Many endpoints are intentional `501 NOT_IMPLEMENTED` stubs (`s.notImplemented`) until their tables exist — don't assume a route works from its presence.

## Auth

- JWT HS256. Access token (15 min) via `Authorization: Bearer`; refresh token (7 days) in httpOnly cookie `refresh_token` scoped to `/api/v1/auth`. Refresh sessions are stored in `sesiones_refresco` for rotation/revocation.
- Role gate: `s.require(w, r, "Administrador", ...)`. Roles are `Administrador`, `Agente`, `Servicio`.
- Service/worker callers use header `X-Service-Token` (== `SERVICE_TOKEN`) mapping to role `Servicio`.
- WhatsApp webhook: `GET` verifies `hub.verify_token`; `POST` requires `X-Webhook-Token`, both == `SERVICE_TOKEN`.

## DB model conventions

- UUID PKs, table-per-subclass inheritance (`personas` -> `clientes`/`usuarios`), plural Spanish table names, soft delete (`deleted_at`) with status set to `Oculto` on property delete, aggregate read model in `sesiones_chat.historial_mensajes` (JSONB).

## Deploy (Render)

Render's default Go build fails (`no Go files` at root). Either set build `go build -o bin/api ./cmd/api` + start `./bin/api`, or use the provided `Dockerfile`/`render.yaml` Blueprint (recommended). App binds `$PORT` automatically. Required env: `DATABASE_URL`, `JWT_SECRET`, `SERVICE_TOKEN`, `COOKIE_SECURE=true`, plus CORS (`ALLOWED_ORIGINS`, `MEDIA_BASE_URL`) and R2 vars (`R2_*`) for image uploads. See `README.md`.
