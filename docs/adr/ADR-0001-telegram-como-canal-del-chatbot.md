# ADR-0001 — Adopción de Telegram como canal del chatbot AdelAI

- **Estado:** Aceptado
- **Fecha:** 2026-11-15
- **Reemplaza:** requisito original de integrar AdelAI con WhatsApp Business Cloud API.
- **Decisores:** alumno del TFI (desarrollo), tutor empresarial (Adelson Servicios Inmobiliarios).

## Contexto

El alcance original del Trabajo Final Integrador contemplaba a AdelAI (el
asistente conversacional de la inmobiliaria) integrado con **WhatsApp**, a
través de la WhatsApp Business Cloud API o un proveedor de servicios de negocio
(BSP) como Twilio. Durante la implementación surgió un bloqueo operativo
insalvable en el plazo disponible:

- La cuenta de Meta Business necesaria para operar la WhatsApp Business Cloud API
  **no pudo completar la verificación de dos factores (2FA)** exigida por la
  plataforma, lo que impide tanto el alta del número como la obtención de un
  token de acceso de producción.
- Aun superado el bloqueo, el canal de WhatsApp impone costes por conversación,
  plantillas de mensaje preaprobadas y una ventana de respuesta de 24 horas para
  mensajes salientes fuera de plantilla, además de la dependencia de un número
  telefónico verificado.

En paralelo, el backend ya contaba con una capa de ingesta de mensajes y una
abstracción de canal (`ChatProvider`) pensada para desacoplar el dominio del
proveedor de mensajería, lo que redujo significativamente el coste de cambiar de
canal.

## Requerimiento modificado

El canal oficial de comunicación de AdelAI pasa a ser **Telegram Bot API**.
WhatsApp deja de ser un canal de producción del proyecto y se conserva únicamente
como código legado/inactivo.

## Decisión

Adoptar **Telegram Bot API** como canal de mensajería del chatbot:

- Implementación detrás de la interfaz `ChatProvider`, junto a un pipeline
  compartido (`handleChatWebhook`) que resuelve alta de cliente, sesión,
  deduplicación y persistencia de mensajes de forma agnóstica al canal.
- Verificación del webhook mediante la cabecera `X-Telegram-Bot-Api-Secret-Token`
  comparada con `TELEGRAM_WEBHOOK_SECRET` (cadena simple `A-Z a-z 0-9 _ -`,
  independiente del token del bot).
- Envío de respuestas con `TELEGRAM_TOKEN` vía `sendMessage`.
- Ruta expuesta: `POST /api/v1/webhooks/telegram`.
- Deduplicación por `update_id`; identidad externa del cliente por `chat.id`.
- El código de WhatsApp (`internal/httpapi/whatsapp.go` y sus rutas) se mantiene
  en el repositorio pero queda **deprecado** y sin uso en producción.

## Justificación

1. **Elimina el bloqueo operativo**: Telegram Bot API no exige 2FA sobre una
   cuenta de Meta, verificación de negocio ni número telefónico verificado.
2. **Menor fricción y coste**: sin pago por conversación, sin aprobación previa
   de plantillas ni ventana de 24 horas; registro del webhook mediante un único
   `setWebhook` con `secret_token`.
3. **Ajuste al plazo del TFI**: permite demostrar el flujo completo
   (ingesta → persistencia → respuesta) en el cronograma de 16 semanas.
4. **Bajo coste de cambio**: la decisión se apoya en el patrón Adapter ya
   contemplado en la arquitectura, por lo que el dominio y el esquema de datos
   no se ven afectados por el proveedor concreto.
5. **Seguridad equivalente**: verificación de origen con token secreto en
   cabecera (comparación en tiempo constante) y deduplicación idempotente.

## Alternativas consideradas

| Alternativa | Motivo de descarte |
|---|---|
| WhatsApp Business Cloud API directa | Bloqueo de 2FA/verificación de Meta; costes, plantillas y ventana de 24 h. |
| BSP de WhatsApp (Twilio, 360dialog, Infobip, Vonage, Bird) | Añaden un tercero de pago y mantienen la dependencia de Meta y del número verificado. |
| WAHA + n8n | Introduce una infraestructura adicional de auto-hospedaje no justificada para el alcance. |
| Solo chat web (sin canal de mensajería) | No cubre el requisito de atención por un canal de mensajería frecuente. |

## Consecuencias

**Positivas**

- Canal operativo sin bloqueos externos y sin coste por conversación.
- Flujo end-to-end demostrable dentro del cronograma.
- La abstracción `ChatProvider` habilita sumar otros canales (o recuperar
  WhatsApp) sin modificar el núcleo.

**Negativas**

- Los clientes deben disponer de Telegram (penetración menor que WhatsApp en el
  mercado objetivo).
- La identidad del cliente se basa en el `chat.id` de Telegram y no en un número
  telefónico real, por lo que el teléfono puede no estar disponible en el CRM.

**Neutras**

- El esquema de `sesiones_chat` no registra canal; el canal se conserva en el
  `metadata` de cada mensaje. El enum `canal_origen_enum` de `consultas` sigue
  siendo `WhatsApp, Portal Web, Manual` y requiere una migración para incorporar
  `Telegram` (ver Impacto).

## Migración / Impacto

- **Sin cambio de esquema** para la ingesta y persistencia de mensajes.
- **Pendiente**: agregar `'Telegram'` a `canal_origen_enum`
  (`ALTER TYPE ... ADD VALUE 'Telegram'`). `database/seed.sql` se mantiene sin
  cambios: su fila demo con `canal_origen = 'WhatsApp'` se conserva como dato
  histórico de desarrollo.
- **Configuración**: nuevas variables `TELEGRAM_TOKEN` y `TELEGRAM_WEBHOOK_SECRET`
  (`.env`, `render.yaml`); `WHATSAPP_APP_SECRET` queda como variable legada.
- **Documentación**: actualizados los documentos del proyecto, el contrato de API,
  el diccionario de datos y este registro (ver `docs/CHANGELOG.md`).

## Referencias

- Código: `internal/httpapi/provider.go` (abstracción), `internal/httpapi/telegram.go`
  (adaptador), `internal/httpapi/server.go` (ruta), `internal/config/config.go`.
- Commits: `041562f` (abstracción + adaptador), `de7c7c8` (loop de eco),
  `af96f26` (separación de secret), `5105a57` (cast en dedupe).
- Documentos: `docs/project/project_idea.md`, `docs/project/objetivos.txt`,
  `docs/project/idea_del_proyecto.txt`, `docs/api/api_documentation.md`,
  `docs/database/DATABASE.md`.
