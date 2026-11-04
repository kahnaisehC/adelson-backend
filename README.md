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

Para bases existentes, aplicar las migraciones pendientes en orden:

```bash
psql "$DATABASE_URL" -f database/migrations/001_public_site.sql
psql "$DATABASE_URL" -f database/migrations/002_admin_panel.sql
```

Opcionalmente cargar datos de ejemplo (un agente y ocho inmuebles con imágenes):

```bash
psql "$DATABASE_URL" -f database/seed.sql
```

Para sincronizar una base de producción en Neon con los ocho inmuebles demo, las imágenes de Unsplash y la migración del sitio público:

```bash
psql "$DATABASE_URL" -f database/neon_production_sync.sql
```

El script genera una contraseña aleatoria para el agente demo y es idempotente. Hacer backup o crear una branch en Neon antes de ejecutarlo. **No volver a ejecutarlo después de crear el administrador real**, porque rota contraseñas y recrea el agente demo.

Crear o actualizar el administrador inicial y reasignar la cartera demo:

```bash
ADMIN_EMAIL='<correo-del-administrador>' \
ADMIN_PASSWORD='una-contraseña-segura' \
go run ./cmd/create-admin -nombre Marcelo -apellido Benato
```

El comando crea el usuario `Administrador`, reasigna los inmuebles y consultas del agente demo y desactiva `agente@adelson.example` sin eliminarlo. Se puede repetir para actualizar la contraseña o los datos del administrador.

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

El panel administrativo agrega:

- Sesiones de refresh con rotación y revocación.
- Login con límite de intentos y verificación de cuenta activa.
- Permisos por cartera para agentes.
- Dashboard, consultas con asignación y usuarios.
- Subida directa a Cloudflare R2 con URLs prefirmadas y borrado de objetos.
- Restauración de inmuebles eliminados.

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
| `MEDIA_BASE_URL` | Base pública de imágenes servida por el Worker, por ejemplo `https://inmobiliariafront.joaquinr-val.workers.dev/media`. |
| `R2_ACCOUNT_ID` | ID de la cuenta de Cloudflare. |
| `R2_ACCESS_KEY_ID` | Access key del token de API de R2. |
| `R2_SECRET_ACCESS_KEY` | Secret del token de API de R2. |
| `R2_BUCKET` | Bucket privado, por defecto `adelson-inmuebles`. |
| `R2_PRESIGN_TTL` | Segundos de validez de las URLs de subida (300 por defecto). |
| `R2_MAX_UPLOAD_BYTES` | Tamaño máximo por imagen (8 MB por defecto). |

Requiere conexión a una instancia PostgreSQL accesible desde Render (p. ej. Neon o Render Postgres) con el esquema de `database/setup.sql` y las migraciones aplicadas. También requiere un bucket R2 privado con CORS para `PUT` desde el dominio del Worker y `http://localhost:5173`.
