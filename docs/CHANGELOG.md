# Registro de cambios

## 2026-11-15 — Cambio de requerimiento: Telegram reemplaza a WhatsApp como canal del chatbot

**Decisión:** ADR-0001 (`docs/adr/ADR-0001-telegram-como-canal-del-chatbot.md`).

**Motivo:** bloqueo de verificación (2FA) de la cuenta de Meta requerida por la
WhatsApp Business Cloud API, sumado a costes por conversación, plantillas
preaprobadas y ventana de 24 horas. Se adopta Telegram Bot API como canal oficial
de AdelAI, apoyado en la abstracción `ChatProvider` existente.

**Documentos actualizados:**

- `docs/project/project_idea.md`
- `docs/project/objetivos.txt`
- `docs/project/idea_del_proyecto.txt`
- `docs/api/api_documentation.md`
- `docs/database/DATABASE.md`
- `README.md`
- `AGENTS.md`

**Configuración afectada:**

- Nuevas: `TELEGRAM_TOKEN`, `TELEGRAM_WEBHOOK_SECRET`.
- Legada (sin uso en producción): `WHATSAPP_APP_SECRET`, `SERVICE_TOKEN`
  (como `hub.verify_token` del webhook de WhatsApp).

**Pendiente (fuera de este cambio):**

- Migración SQL para agregar `'Telegram'` a `canal_origen_enum` y actualizar
  `database/seed.sql`.
- Retiro definitivo del código de WhatsApp (`internal/httpapi/whatsapp.go`).
- Actualización de menciones de WhatsApp en el repositorio frontend.
