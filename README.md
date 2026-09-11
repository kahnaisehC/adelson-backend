# Adelson Backend

Backend REST en Go para AdelsonWeb y AdelAI. La implementación sigue `docs/api/api_documentation.md` y utiliza PostgreSQL definido en `database/setup.sql`.

## Requisitos

- Go 1.26 o posterior.
- PostgreSQL compatible con el esquema del proyecto.

## Configuración

Copiar `.env.example` a un archivo de entorno y definir las variables:

```bash
export ADDR=:8080
export DATABASE_URL='postgres://user:password@localhost:5432/adelson?sslmode=disable'
export JWT_SECRET='use-a-long-random-secret'
export SERVICE_TOKEN='use-a-secret-for-internal-services'
export COOKIE_SECURE=false
```

Aplicar el esquema antes de iniciar la API:

```bash
psql "$DATABASE_URL" -f database/setup.sql
```

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
- Consultas internas y consultas públicas desde un inmueble.
- Sesiones de chat, mensajes y transiciones de handoff.
- Verificación básica de webhook de WhatsApp.

Los endpoints de contratos, seguimientos, recomendaciones persistidas, notificaciones, favoritos y auditoría responden `501 Not Implemented` hasta que se agreguen las tablas indicadas en `docs/api/api_documentation.md`.

## Pruebas y formato

```bash
gofmt -w cmd internal
go test ./...
go vet ./...
```
