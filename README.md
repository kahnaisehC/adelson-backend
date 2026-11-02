# Adelson Backend

Backend REST en Go para AdelsonWeb y AdelAI. La implementación sigue `docs/api/api_documentation.md` y utiliza PostgreSQL definido en `database/setup.sql`.

## Requisitos

- Go 1.26 o posterior.
- PostgreSQL compatible con el esquema del proyecto.

## Configuración

Crear `.env` a partir de `.env.example` y completar las variables:

```bash
cp .env.example .env
```

`config.Load()` carga `.env` automáticamente. Las variables exportadas en el entorno tienen prioridad sobre los valores del archivo.

Aplicar el esquema antes de iniciar la API:

```bash
psql "$DATABASE_URL" -f database/setup.sql
```

Si no tienes PostgreSQL instalado, el script usa Podman o Docker para crear una instancia local persistente:

```bash
./scripts/setup-db.sh
```

El esquema se aplica automáticamente la primera vez que se crea el volumen.

Para bases existentes, aplicar las migraciones pendientes:

```bash
psql "$DATABASE_URL" -f database/migrations/001_public_site.sql
```

Opcionalmente cargar datos de ejemplo (un agente y ocho inmuebles con imágenes):

```bash
psql "$DATABASE_URL" -f database/seed.sql
```

Reemplazar `pub-xxxxxxxx.r2.dev` en `database/seed.sql` por el dominio público real del bucket R2.

Iniciar:

```bash
go run ./cmd/api
```

Comprobar disponibilidad:

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Estado de implementación

Implementado con PostgreSQL:

- Autenticación de usuarios internos con JWT y bcrypt.
- Gestión de usuarios internos con roles `Administrador` y `Agente`.
- Inmuebles, búsqueda, creación, actualización, borrado lógico e imágenes.
- Clientes, teléfonos, consentimiento y preferencias.
- Consultas internas, consultas públicas desde un inmueble y consulta general desde `/contacto`.
- Sesiones de chat, mensajes y transiciones de handoff.
- Verificación básica de webhook de WhatsApp.

Los endpoints de contratos, seguimientos, recomendaciones persistidas, notificaciones, favoritos y auditoría responden `501 Not Implemented` hasta que se agreguen las tablas indicadas en `docs/api/api_documentation.md`.

## Pruebas y formato

```bash
gofmt -w cmd internal
go test ./...
go vet ./...
```

## Deploy en Render

El `main` vive en `./cmd/api`, así que Render no puede usar su build de Go por defecto (compila el directorio raíz y falla con `no Go files`). Hay dos formas de desplegar:

### Opción A: Build nativo (rápido, servicio existente)

En el dashboard de Render, en tu servicio, configurar:

- **Build Command:** `go build -o bin/api ./cmd/api`
- **Start Command:** `./bin/api`

Opcionalmente definir `GO_VERSION=1.26.5` en el entorno. El servicio escucha en `$PORT` de Render automáticamente.

### Opción B: Docker (recomendada)

El repositorio incluye `Dockerfile` y `render.yaml`. Crear el servicio con **Blueprint** (`New +`) desde `render.yaml`, o un **Web Service** con runtime *Docker* y `dockerContext: .`.

Variables de entorno requeridas:

| Variable | Descripción |
|---|---|
| `DATABASE_URL` | Cadena de conexión a PostgreSQL (immutable). |
| `JWT_SECRET` | Secreto del JWT; `render.yaml` lo genera automáticamente. |
| `SERVICE_TOKEN` | Token de servicio para workers/webhooks; `render.yaml` lo genera automáticamente. |
| `COOKIE_SECURE` | `true` en producción (HTTPS). |
| `ALLOWED_ORIGINS` | Orígenes permitidos por CORS, separados por coma. Incluir el dominio de Cloudflare Workers. |

Requiere conexión a una instancia PostgreSQL accesible desde Render (p. ej. Neon o Render Postgres) con el esquema de `database/setup.sql` aplicado.
