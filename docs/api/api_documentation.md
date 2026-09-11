
# API REST - AdelsonWeb y AdelAI

Documento de referencia para el backend de la plataforma inmobiliaria de Adelson Servicios Inmobiliarios.

**Estado:** contrato inicial de la API  
**Versión:** `v1`  
**Fuentes:** `docs/project/project_idea.md`, `docs/project/objetivos.txt` y `database/setup.sql`

La API es la única puerta de entrada al dominio. El frontend, AdelAI y los workers no deben acceder directamente a PostgreSQL. Todas las operaciones pasan por validaciones, autorización, reglas de negocio y repositorios parametrizados.

## 1. Convenciones generales

### URL base

```text
https://api.adelson.example/api/v1
```

El dominio es ilustrativo y debe reemplazarse durante el despliegue. Todas las rutas de este documento son relativas a la URL base.

### Formato

- Solicitudes y respuestas usan `application/json`, salvo cargas de archivos.
- Los nombres de propiedades JSON usan `camelCase`.
- Los nombres de las tablas y columnas internas conservan `snake_case`.
- Los identificadores son UUID.
- Las fechas se serializan en ISO 8601 UTC, por ejemplo `2026-09-10T14:30:00Z`.
- Los valores de los enumerados se exponen exactamente como están definidos en PostgreSQL.
- `contrasenaHash` nunca forma parte de una respuesta.

### Encabezados

```http
Authorization: Bearer <access-token>
Content-Type: application/json
Accept: application/json
X-Request-ID: <uuid-opcional>
```

`X-Request-ID` se genera si el cliente no lo envía y debe aparecer en los logs y en la respuesta. Las operaciones que puedan reintentarse deben aceptar además:

```http
Idempotency-Key: <clave-unica-del-cliente>
```

La clave es obligatoria para envíos de mensajes, procesamiento de webhooks y cualquier operación que cree una notificación externa.

### Respuestas exitosas

Respuesta de un recurso:

```json
{
  "data": {
    "id": "6b3c4a5e-5d91-4d71-9e91-7ef89d0e8f11"
  },
  "meta": {
    "requestId": "b9a0f45d-0b6a-4da2-9b1f-dc7a7cf0a2a1"
  }
}
```

Respuesta paginada:

```json
{
  "data": [],
  "meta": {
    "page": 1,
    "pageSize": 20,
    "total": 0,
    "totalPages": 0,
    "requestId": "b9a0f45d-0b6a-4da2-9b1f-dc7a7cf0a2a1"
  }
}
```

Parámetros de paginación comunes:

| Parámetro | Tipo | Default | Restricción |
|---|---:|---:|---|
| `page` | integer | `1` | Mayor o igual que `1`. |
| `pageSize` | integer | `20` | Entre `1` y `100`. |
| `sort` | string | Recurso | Solo campos incluidos en la lista de ordenamiento de cada endpoint. |
| `order` | string | `desc` | `asc` o `desc`. |

### Errores

Todas las respuestas de error usan la siguiente estructura:

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "La solicitud contiene datos inválidos.",
    "details": [
      {
        "field": "precio",
        "reason": "Debe ser mayor que cero."
      }
    ],
    "requestId": "b9a0f45d-0b6a-4da2-9b1f-dc7a7cf0a2a1"
  }
}
```

| Código HTTP | Código API | Uso |
|---:|---|---|
| `400` | `INVALID_REQUEST` | JSON mal formado o parámetros incompatibles. |
| `401` | `UNAUTHENTICATED` | Falta autenticación o el token es inválido. |
| `403` | `FORBIDDEN` | El rol autenticado no tiene permiso. |
| `404` | `NOT_FOUND` | El recurso no existe o fue eliminado lógicamente. |
| `409` | `CONFLICT` | DNI, correo, idempotencia o estado duplicado. |
| `422` | `VALIDATION_ERROR` | Datos sintácticamente válidos pero inválidos para el dominio. |
| `429` | `RATE_LIMITED` | Se superó el límite de solicitudes. |
| `500` | `INTERNAL_ERROR` | Error no controlado; no se devuelven detalles internos. |
| `502` | `UPSTREAM_ERROR` | Falló WhatsApp, almacenamiento u otro proveedor externo. |

## 2. Autenticación y autorización

### Tokens

Los usuarios internos se autentican con un access token JWT de corta duración. El refresh token debe rotarse y almacenarse en una cookie `HttpOnly`, `Secure` y `SameSite=Lax`, o en un almacén seguro equivalente. Nunca debe guardarse en `localStorage`.

```http
Authorization: Bearer eyJhbGciOi...
```

El token contiene, como mínimo, `sub`, `role`, `iat`, `exp` y `jti`. La API comprueba que la persona no esté eliminada y que el usuario tenga estado operativo `Activo`.

### Roles

| Actor | Permisos principales |
|---|---|
| Público | Consultar inmuebles disponibles y crear una consulta limitada. Está sujeto a rate limiting. |
| `Agente` | Gestionar sus inmuebles, clientes, consultas, preferencias y conversaciones asignadas. |
| `Administrador` | Acceso total al panel, incluyendo usuarios y recursos de todos los agentes. |
| Servicio AdelAI/worker | Acceso máquina a máquina con scopes explícitos. No puede ejecutar SQL ni omitir validaciones. |

El cliente final no utiliza el rol `usuario`: sus operaciones se realizan mediante un flujo público limitado, un token de sesión de cliente cuando se implemente, o el servicio AdelAI.

## 3. Valores enumerados

La API conserva los valores del esquema para evitar traducciones ambiguas.

| Campo | Valores permitidos |
|---|---|
| `estadoLead` | `Nuevo`, `Contactado`, `En Negociación`, `Cerrado`, `Inactivo` |
| `tipoClientePrincipal` | `Buscando Alquiler`, `Buscando Comprar`, `Propietario` |
| `rol` | `Administrador`, `Agente` |
| `estadoOperativo` | `Activo`, `Inactivo`, `Suspendido` |
| `canalOrigen` | `WhatsApp`, `Portal Web`, `Manual` |
| `estadoSeguimiento` | `Pendiente`, `Contactado`, `Visita Agendada`, `Descartado`, `Cerrado` |
| `estadoSesion` | `Atendida por IA`, `Esperando Humano`, `Atendida por Humano`, `Cerrada` |
| `tipoOperacion` | `Venta`, `Alquiler`, `Alquiler Temporario` |
| `tipoInmueble` | `Casa`, `Departamento`, `Terreno`, `Local Comercial`, `Galpón` |
| `estado` | `Disponible`, `Reservado`, `Vendido`, `Alquilado`, `Oculto` |
| `moneda` | `Pesos`, `Dólares` |

## 4. Autenticación y usuarios

### Endpoints

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| `POST` | `/auth/login` | Público | Inicia sesión de un administrador o agente. |
| `POST` | `/auth/refresh` | Refresh token | Rota el access token. |
| `POST` | `/auth/logout` | Autenticado | Revoca la sesión y el refresh token. |
| `GET` | `/auth/me` | Autenticado | Devuelve el usuario autenticado. |
| `GET` | `/usuarios` | Administrador | Lista usuarios internos. |
| `POST` | `/usuarios` | Administrador | Crea una persona y su perfil de usuario en una transacción. |
| `GET` | `/usuarios/{usuarioId}` | Administrador o usuario propio | Consulta un usuario. |
| `PATCH` | `/usuarios/{usuarioId}` | Administrador o usuario propio | Actualiza datos permitidos. |

No se elimina físicamente un usuario. Para deshabilitarlo se usa `estadoOperativo: "Inactivo"` o `"Suspendido"`.

### Inicio de sesión

`POST /auth/login`

```json
{
  "correoElectronico": "agente@adelson.example",
  "contrasena": "SecretoSeguro"
}
```

Respuesta `200`:

```json
{
  "data": {
    "accessToken": "eyJhbGciOi...",
    "tokenType": "Bearer",
    "expiresIn": 900,
    "usuario": {
      "id": "6b3c4a5e-5d91-4d71-9e91-7ef89d0e8f11",
      "nombre": "Ana",
      "apellido": "Pérez",
      "correoElectronico": "agente@adelson.example",
      "rol": "Agente",
      "estadoOperativo": "Activo"
    }
  }
}
```

Respuestas `401` no deben revelar si el correo existe.

### Crear usuario

`POST /usuarios`

```json
{
  "nombre": "Ana",
  "apellido": "Pérez",
  "dni": "30111222",
  "correoElectronico": "ana@adelson.example",
  "contrasena": "UnaContrasenaLarga",
  "rol": "Agente"
}
```

La contraseña se hashea con Argon2id o bcrypt antes de persistirse. La operación crea `personas` y `usuarios` con el mismo UUID y falla completa si alguna parte no puede guardarse.

### Actualizar usuario

`PATCH /usuarios/{usuarioId}` acepta únicamente los campos autorizados por el actor:

```json
{
  "nombre": "Ana María",
  "correoElectronico": "ana.maria@adelson.example"
}
```

Un administrador puede cambiar `rol` y `estadoOperativo`. Un usuario puede actualizar sus datos personales y contraseña mediante un flujo específico; no puede elevar sus privilegios. `ultimoAcceso` lo actualiza exclusivamente el servidor al iniciar sesión.

## 5. Catálogo de inmuebles

La tabla `inmuebles` es la fuente de los avisos publicados. Las respuestas públicas excluyen registros con `deletedAt` y, por defecto, solo muestran `estado: "Disponible"`.

### Endpoints

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| `GET` | `/inmuebles` | Público | Búsqueda y filtrado del catálogo. |
| `POST` | `/inmuebles` | Agente/Administrador | Crea un inmueble. |
| `GET` | `/inmuebles/{inmuebleId}` | Público o interno | Consulta el detalle y sus imágenes. |
| `PATCH` | `/inmuebles/{inmuebleId}` | Agente propietario/Administrador | Actualiza parcialmente un inmueble. |
| `DELETE` | `/inmuebles/{inmuebleId}` | Agente propietario/Administrador | Ejecuta borrado lógico. |
| `POST` | `/inmuebles/{inmuebleId}/restaurar` | Administrador | Restaura un inmueble eliminado lógicamente. |
| `GET` | `/inmuebles/{inmuebleId}/imagenes` | Público o interno | Lista imágenes ordenadas. |
| `POST` | `/inmuebles/{inmuebleId}/imagenes` | Agente propietario/Administrador | Registra una imagen almacenada externamente. |
| `PATCH` | `/inmuebles/{inmuebleId}/imagenes/{imagenId}` | Agente propietario/Administrador | Cambia orden o portada. |
| `DELETE` | `/inmuebles/{inmuebleId}/imagenes/{imagenId}` | Agente propietario/Administrador | Elimina la referencia a la imagen. |

### Búsqueda y filtros

`GET /inmuebles`

Parámetros soportados:

| Parámetro | Tipo | Descripción |
|---|---|---|
| `tipoOperacion` | string | `Venta`, `Alquiler` o `Alquiler Temporario`. |
| `tipoInmueble` | string | Tipo de inmueble del enumerado. |
| `estado` | string | Solo disponible para usuarios internos; el público queda limitado a `Disponible`. |
| `moneda` | string | `Pesos` o `Dólares`. |
| `precioMin` / `precioMax` | number | Rango en la moneda indicada. No se convierten monedas automáticamente. |
| `cantHabitacionesMin` | integer | Cantidad mínima de habitaciones. |
| `cantBanosMin` | integer | Cantidad mínima de baños. |
| `superficieTotalMin` / `superficieTotalMax` | number | Rango de superficie total. |
| `ciudad` | string | Búsqueda por ciudad. |
| `barrio` | string | Búsqueda por barrio. |
| `q` | string | Texto libre sobre título, descripción y ubicación. |
| `caracteristicas` | object/string | Filtros sobre claves de `caracteristicas` JSONB. |
| `agenteId` | UUID | Solo administrador; filtra por agente responsable. |
| `incluirEliminados` | boolean | Solo administrador; default `false`. |
| `page`, `pageSize`, `sort`, `order` | - | Paginación y ordenamiento comunes. |

Ejemplo:

```http
GET /api/v1/inmuebles?tipoOperacion=Alquiler&tipoInmueble=Departamento&moneda=Dólares&precioMax=800&cantHabitacionesMin=2&barrio=Centro&page=1&pageSize=20
```

Las consultas combinadas se implementan con parámetros preparados. `precioMin` y `precioMax` deben interpretarse en la misma moneda; si se envía un rango de precio sin `moneda`, la API responde `422`.

### Crear inmueble

`POST /inmuebles`

```json
{
  "agenteId": "6b3c4a5e-5d91-4d71-9e91-7ef89d0e8f11",
  "titulo": "Departamento luminoso en el Centro",
  "descripcion": "Dos habitaciones, balcón y cochera.",
  "tipoOperacion": "Alquiler",
  "tipoInmueble": "Departamento",
  "estado": "Disponible",
  "precio": 800.00,
  "moneda": "Dólares",
  "montoExpensas": 95000.00,
  "cantAmbientes": 3,
  "cantHabitaciones": 2,
  "cantBanos": 1,
  "cantCocheras": 1,
  "superficieTotal": 70.00,
  "superficieCubierta": 65.00,
  "calle": "San Martín",
  "numero": "123",
  "barrio": "Centro",
  "ciudad": "Rafaela",
  "latitud": -31.2500,
  "longitud": -61.4867,
  "caracteristicas": {
    "balcon": true,
    "aptoMascotas": true,
    "calefaccion": "Gas"
  }
}
```

Reglas de validación:

- `titulo`, `tipoOperacion`, `tipoInmueble`, `precio` y `moneda` son obligatorios.
- `precio` debe ser mayor que cero y `montoExpensas` no puede ser negativo.
- Las cantidades y superficies no pueden ser negativas.
- Las superficies y coordenadas deben ser numéricas válidas.
- Un agente solo puede asignarse a sí mismo o a un inmueble permitido por la política de la agencia. El administrador puede asignar cualquier agente activo.
- El objeto `caracteristicas` debe ser JSON válido y no debe contener credenciales ni datos personales innecesarios.

### Actualizar y eliminar

`PATCH /inmuebles/{inmuebleId}` acepta un subconjunto de los campos de creación. No permite modificar `id`, `createdAt` ni `deletedAt` directamente. La API actualiza `updatedAt` en cada cambio.

`DELETE /inmuebles/{inmuebleId}` establece `deletedAt` y normalmente cambia el estado público a `Oculto`. No se ejecuta `DELETE FROM inmuebles` desde la API.

### Imágenes

Los archivos se almacenan en R2 u otro proveedor; PostgreSQL conserva solamente la referencia `urlArchivo`.

`POST /inmuebles/{inmuebleId}/imagenes`

```json
{
  "urlArchivo": "https://cdn.adelson.example/inmuebles/6b3c.../frente.webp",
  "ordenVisualizacion": 1,
  "esPortada": true
}
```

El backend debe validar que la URL haya sido emitida por el adaptador de almacenamiento. Solo puede existir una portada efectiva por inmueble; al marcar una nueva portada se desmarca la anterior dentro de una transacción.

## 6. CRM y clientes

`clientes` utiliza el mismo UUID que `personas`. Crear, modificar o eliminar un cliente requiere actualizar la superclase y la subclase de forma transaccional.

### Endpoints de clientes

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| `GET` | `/clientes` | Agente/Administrador | Lista y filtra clientes. |
| `POST` | `/clientes` | Agente/Administrador/Servicio IA | Crea un cliente y su persona. |
| `GET` | `/clientes/{clienteId}` | Agente/Administrador/Servicio IA | Consulta el perfil. |
| `PATCH` | `/clientes/{clienteId}` | Agente/Administrador/Servicio IA autorizado | Actualiza datos permitidos. |
| `DELETE` | `/clientes/{clienteId}` | Administrador | Borrado lógico de la persona. |
| `PATCH` | `/clientes/{clienteId}/consentimiento` | Agente/Administrador/Servicio IA autorizado | Activa o revoca notificaciones. |
| `GET` | `/clientes/{clienteId}/telefonos` | Agente/Administrador/Servicio IA | Lista teléfonos. |
| `POST` | `/clientes/{clienteId}/telefonos` | Agente/Administrador/Servicio IA autorizado | Agrega un teléfono. |
| `PATCH` | `/clientes/{clienteId}/telefonos/{telefonoId}` | Agente/Administrador/Servicio IA autorizado | Actualiza un teléfono. |
| `DELETE` | `/clientes/{clienteId}/telefonos/{telefonoId}` | Agente/Administrador | Elimina un teléfono. |

Filtros de `GET /clientes`:

`q`, `estadoLead`, `tipoClientePrincipal`, `consentimientoNotificaciones`, `correoElectronico`, `createdFrom`, `createdTo`, `page`, `pageSize`, `sort` y `order`.

### Crear cliente

`POST /clientes`

```json
{
  "nombre": "María",
  "apellido": "Gómez",
  "dni": "32999888",
  "correoElectronico": "maria@example.com",
  "contrasena": "opcional-si-se-habilita-portal",
  "telefonos": [
    {
      "numero": "+5493492123456",
      "esPrincipal": true
    }
  ],
  "consentimientoNotificaciones": true,
  "tipoClientePrincipal": "Buscando Alquiler"
}
```

`contrasena` no se devuelve. Si el cliente solo se utiliza para CRM o WhatsApp y no tendrá login propio, el servicio debe generar internamente un hash aleatorio no reutilizable para satisfacer la columna `contrasena_hash NOT NULL`.

El correo y el DNI son únicos. La API responde `409 CONFLICT` si ya existen. No debe utilizarse un `UPDATE` directo para cambiar el UUID de una persona.

### Actualizar cliente

`PATCH /clientes/{clienteId}` permite, entre otros, los siguientes campos:

```json
{
  "nombre": "María",
  "apellido": "Gómez",
  "correoElectronico": "maria.nueva@example.com",
  "estadoLead": "Contactado",
  "tipoClientePrincipal": "Buscando Comprar"
}
```

`fechaRegistro`, `fechaUltimaInteraccion` y `deletedAt` los controla el servidor. `fechaUltimaInteraccion` se actualiza al registrar una interacción válida, no ante un `GET`.

### Consentimiento

`PATCH /clientes/{clienteId}/consentimiento`

```json
{
  "consentimientoNotificaciones": false,
  "motivo": "Solicitud del cliente",
  "canal": "WhatsApp"
}
```

Cuando el valor es `false`, los workers deben bloquear recomendaciones y recordatorios para ese cliente. La tabla actual solo persiste el booleano; el canal, la fecha y el historial de revocaciones requieren la ampliación indicada en la sección de pendientes de esquema.

### Teléfonos

`POST /clientes/{clienteId}/telefonos`

```json
{
  "numero": "+5493492123456",
  "esPrincipal": true
}
```

No se permiten números duplicados para la misma persona. Al marcar un número como principal, el servicio debe desmarcar los demás dentro de una transacción.

## 7. Preferencias y búsquedas guardadas

En la versión actual, el recurso de búsqueda guardada se persiste en `preferencias_busquedas`. Cada cliente puede tener varias preferencias.

### Endpoints

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| `GET` | `/clientes/{clienteId}/preferencias` | Agente/Administrador/Servicio IA autorizado | Lista preferencias del cliente. |
| `POST` | `/clientes/{clienteId}/preferencias` | Agente/Administrador/Servicio IA autorizado | Crea una preferencia. |
| `GET` | `/clientes/{clienteId}/preferencias/{preferenciaId}` | Agente/Administrador/Servicio IA autorizado | Consulta una preferencia. |
| `PATCH` | `/clientes/{clienteId}/preferencias/{preferenciaId}` | Agente/Administrador/Servicio IA autorizado | Actualiza parcialmente una preferencia. |
| `DELETE` | `/clientes/{clienteId}/preferencias/{preferenciaId}` | Agente/Administrador/Servicio IA autorizado | Elimina la preferencia. |
| `GET` | `/clientes/{clienteId}/recomendaciones` | Agente/Administrador/Servicio IA | Calcula inmuebles compatibles. |

### Crear preferencia

`POST /clientes/{clienteId}/preferencias`

```json
{
  "rangoPrecioMin": 500.00,
  "rangoPrecioMax": 800.00,
  "moneda": "Dólares",
  "tipoOperacion": "Alquiler",
  "tipoInmueble": "Departamento",
  "cantidadDormitoriosMin": 2,
  "zonaPreferida": "Centro"
}
```

Todos los campos de criterio son opcionales, pero debe existir al menos un criterio útil. Reglas adicionales:

- `rangoPrecioMin` no puede superar `rangoPrecioMax`.
- Si se informa un rango de precio, se debe informar `moneda`.
- `cantidadDormitoriosMin` debe ser mayor o igual que cero.
- La búsqueda no debe exponer clientes de otra cuenta o agente sin autorización.

La tabla no incluye cantidad mínima de ambientes, nombre, estado activo, fecha de modificación ni configuración de frecuencia. Aunque el proyecto requiere filtrar por ambientes, `preferencias_busquedas` solo permite persistir dormitorios; se necesita una migración para agregar, por ejemplo, `cantidad_ambientes_min`. Los atributos adicionales deben agregarse antes de tratarlos como parte estable de la API.

### Recomendaciones

`GET /clientes/{clienteId}/recomendaciones` calcula coincidencias sobre inmuebles no eliminados y normalmente con estado `Disponible`. Puede aceptar `preferenciaId`, `limit` y `incluirVistas` cuando se agregue la relación de vistas/notificaciones.

La respuesta debe explicar el criterio de coincidencia sin revelar reglas internas sensibles:

```json
{
  "data": [
    {
      "inmueble": {
        "id": "6b3c4a5e-5d91-4d71-9e91-7ef89d0e8f11",
        "titulo": "Departamento luminoso en el Centro",
        "precio": 800.0,
        "moneda": "Dólares"
      },
      "preferenciaId": "1bc2a12e-6f8e-4f91-8bf1-8de8e18f3c42",
      "coincidencias": ["tipoOperacion", "moneda", "rangoPrecio", "zonaPreferida"]
    }
  ]
}
```

## 8. Consultas y seguimiento comercial

La tabla `consultas` relaciona clientes e inmuebles y registra el canal y estado del contacto.

### Endpoints

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| `GET` | `/consultas` | Agente/Administrador | Lista consultas con filtros. |
| `POST` | `/consultas` | Agente/Administrador/Servicio IA | Registra una consulta con un cliente existente. |
| `POST` | `/inmuebles/{inmuebleId}/consultas` | Público | Registra una consulta desde el portal. |
| `GET` | `/consultas/{consultaId}` | Agente/Administrador | Consulta el detalle. |
| `PATCH` | `/consultas/{consultaId}` | Agente/Administrador | Actualiza estado y notas internas. |

`GET /consultas` acepta `clienteId`, `inmuebleId`, `canalOrigen`, `estadoSeguimiento`, `createdFrom`, `createdTo`, `page`, `pageSize`, `sort` y `order`.

### Consulta interna

`POST /consultas`

```json
{
  "clienteId": "6b3c4a5e-5d91-4d71-9e91-7ef89d0e8f11",
  "inmuebleId": "3e5d8a7a-5c6e-43ce-978e-8ef3c1c5375a",
  "canalOrigen": "Manual",
  "notasInternas": "Solicita visita durante la tarde."
}
```

`fechaHora` y `estadoSeguimiento` se generan con sus valores por defecto si no se envían. Un cambio de estado actualiza `updatedAt`.

### Consulta pública

`POST /inmuebles/{inmuebleId}/consultas`

```json
{
  "nombre": "María",
  "apellido": "Gómez",
  "dni": "32999888",
  "correoElectronico": "maria@example.com",
  "numeroTelefono": "+5493492123456",
  "mensaje": "Quisiera coordinar una visita."
}
```

El servicio crea o encuentra el cliente de forma segura, registra `canalOrigen: "Portal Web"` y no permite que el público escriba `notasInternas` ni modifique el estado. La columna `personas.dni` es obligatoria en el esquema actual; si el formulario público no debe solicitar DNI, se necesita una entidad de leads separada o una migración antes de implementar este contrato.

### Actualizar consulta

`PATCH /consultas/{consultaId}`

```json
{
  "estadoSeguimiento": "Visita Agendada",
  "notasInternas": "Visita confirmada para el viernes a las 17:00."
}
```

El agente solo puede modificar consultas dentro de su ámbito de trabajo; el administrador puede modificar todas.

## 9. Chat, AdelAI y handoff humano

La tabla `sesiones_chat` mantiene una sesión por cliente y un `historialMensajes` JSONB. El API encapsula ese JSON y no permite reemplazarlo directamente: los mensajes se agregan mediante el endpoint de mensajes.

### Endpoints de sesiones

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| `GET` | `/sesiones-chat` | Agente/Administrador/Servicio IA | Lista sesiones y filtra por estado o agente. |
| `POST` | `/sesiones-chat` | Servicio IA/Agente/Administrador | Crea una sesión para un cliente. |
| `GET` | `/sesiones-chat/{sesionId}` | Agente/Administrador/Servicio IA | Consulta estado e historial. |
| `POST` | `/sesiones-chat/{sesionId}/mensajes` | Servicio IA/Agente/Administrador | Agrega un mensaje. |
| `POST` | `/sesiones-chat/{sesionId}/handoff` | Servicio IA/Agente/Administrador | Solicita intervención humana. |
| `POST` | `/sesiones-chat/{sesionId}/tomar-control` | Agente/Administrador | Asigna la sesión al agente. |
| `POST` | `/sesiones-chat/{sesionId}/reanudar-ia` | Agente/Administrador | Devuelve la sesión a AdelAI. |
| `POST` | `/sesiones-chat/{sesionId}/cerrar` | Agente/Administrador/Servicio IA | Cierra la sesión. |

Filtros de `GET /sesiones-chat`: `clienteId`, `agenteAsignadoId`, `estadoSesion`, `fechaDesde`, `fechaHasta`, `page`, `pageSize`, `sort` y `order`.

### Crear sesión

`POST /sesiones-chat`

```json
{
  "clienteId": "6b3c4a5e-5d91-4d71-9e91-7ef89d0e8f11"
}
```

El estado inicial es `Atendida por IA`, `agenteAsignadoId` es `null` y el historial es un arreglo vacío.

### Agregar mensaje

`POST /sesiones-chat/{sesionId}/mensajes`

```json
{
  "remitenteTipo": "ia",
  "contenido": "Encontré tres propiedades que coinciden con tu búsqueda.",
  "providerMessageId": "wamid.HBgL...",
  "estadoEntrega": "enviado",
  "fechaHora": "2026-09-10T14:30:00Z",
  "metadata": {
    "modelo": "adelai",
    "tool": "buscar_propiedades"
  }
}
```

Valores de `remitenteTipo`: `cliente`, `ia`, `agente` y `sistema`. El servidor genera un identificador interno del mensaje, valida el tamaño del contenido y actualiza `fechaUltimaInteraccion`. El `providerMessageId` debe ser único cuando proviene de WhatsApp.

### Estados de handoff

- `handoff`: cambia a `Esperando Humano` y deja `agenteAsignadoId` en `null` hasta que un agente tome la conversación.
- `tomar-control`: requiere un agente autenticado, establece `agenteAsignadoId` y cambia a `Atendida por Humano`.
- `reanudar-ia`: limpia la asignación, cambia a `Atendida por IA` y deja constancia del evento en auditoría.
- `cerrar`: cambia a `Cerrada`; no permite nuevos mensajes salvo una reapertura explícita definida posteriormente.

Estas transiciones deben validarse como máquina de estados y no como un `PATCH` arbitrario.

### Webhooks de WhatsApp

| Método | Ruta | Acceso | Descripción |
|---|---|---|---|
| `GET` | `/webhooks/whatsapp` | WhatsApp | Verifica el webhook con `hub.challenge`. |
| `POST` | `/webhooks/whatsapp` | WhatsApp | Recibe mensajes y eventos de estado. |

El `POST` debe:

1. Validar `X-Hub-Signature-256` con el secreto configurado.
2. Validar el identificador del proveedor y la `Idempotency-Key` o `messageId`.
3. Responder rápidamente `200` o `202`.
4. Encolar el procesamiento sin bloquear la solicitud de WhatsApp.
5. Encontrar o crear el cliente y la sesión.
6. Agregar el mensaje al historial mediante el servicio de chat.

Un mensaje repetido del proveedor no debe crear otra sesión, consulta ni respuesta.

## 10. Herramientas controladas para AdelAI

AdelAI utiliza operaciones explícitas y con scopes, no un endpoint genérico que acepte SQL o nombres de métodos arbitrarios.

| Herramienta | Endpoint utilizado | Escritura | Confirmación |
|---|---|---:|---|
| `buscar_propiedades` | `GET /inmuebles` | No | No. |
| `guardar_preferencias` | `POST /clientes/{clienteId}/preferencias` | Sí | Confirmar criterios antes de guardar si fueron inferidos. |
| `actualizar_datos_cliente` | `PATCH /clientes/{clienteId}` | Sí | Confirmar datos personales sensibles. |
| `consultar_estado_conversacion` | `GET /sesiones-chat/{sesionId}` | No | No. |
| `registrar_fecha_contrato` | `POST /clientes/{clienteId}/contratos` | Sí | Requiere migración de esquema y confirmación. |
| `crear_seguimiento` | `POST /seguimientos` | Sí | Requiere migración de esquema. |
| `solicitar_humano` | `POST /sesiones-chat/{sesionId}/handoff` | Sí | No, la petición del cliente es suficiente. |

El servicio de IA debe autenticarse máquina a máquina, usar un scope mínimo y enviar `X-Request-ID`. Las operaciones de escritura deben pasar por los mismos servicios de dominio que utiliza el panel administrativo.

## 11. Matching, workers y notificaciones

El matching periódico no debe ejecutarse dentro de una solicitud del frontend. Un worker procesa nuevas propiedades y preferencias mediante una cola.

### Endpoint interno de matching

`POST /interno/jobs/matching`

**Acceso:** worker autenticado con service token.  
**Idempotencia:** obligatoria.

```json
{
  "inmuebleId": "3e5d8a7a-5c6e-43ce-978e-8ef3c1c5375a",
  "desde": "2026-09-10T14:00:00Z"
}
```

El worker debe:

- considerar solo propiedades no eliminadas y publicables;
- comparar cada propiedad con las preferencias activas;
- excluir clientes sin `consentimientoNotificaciones`;
- evitar enviar dos veces la misma recomendación mediante una clave de idempotencia;
- encolar el mensaje y no llamar a WhatsApp dentro de la transacción de PostgreSQL;
- registrar éxito o fallo en la infraestructura de notificaciones.

El endpoint puede devolver un resumen:

```json
{
  "data": {
    "inmuebleId": "3e5d8a7a-5c6e-43ce-978e-8ef3c1c5375a",
    "preferenciasEvaluadas": 42,
    "coincidencias": 8,
    "notificacionesEncoladas": 6,
    "omitidasSinConsentimiento": 2
  }
}
```

El endpoint no debe exponerse al frontend ni a AdelAI.

## 12. Endpoints que requieren ampliar el esquema

El proyecto idea incluye contratos, seguimientos, notificaciones, favoritos y auditoría, pero `database/setup.sql` todavía no define tablas para esas entidades. Las siguientes rutas son parte del contrato futuro y deben permanecer marcadas como pendientes hasta agregar migraciones y repositorios.

| Estado | Método y ruta propuesta | Motivo |
|---|---|---|
| Pendiente | `GET/POST/PATCH/DELETE /clientes/{clienteId}/contratos` | No existe tabla de contratos ni fecha estimada de vencimiento. |
| Pendiente | `GET/POST/PATCH /seguimientos` | `consultas` tiene un estado, pero no fecha programada, motivo, resultado ni tarea independiente. |
| Pendiente | `GET /notificaciones`, `POST /interno/notificaciones/procesar` | No existe outbox, estado de entrega, proveedor ni clave de idempotencia. |
| Pendiente | `GET /auditoria` | No existe bitácora inmutable para actor, acción, entidad y valores anteriores/nuevos. |
| Pendiente | `GET/POST/DELETE /clientes/{clienteId}/favoritos/{inmuebleId}` | No existe relación de favoritos. |
| Pendiente | `GET /clientes/{clienteId}/consentimientos` | El booleano actual no guarda canal, fecha, fuente ni revocaciones. |

### Tablas mínimas sugeridas

Antes de habilitar esas rutas se recomienda agregar, como mínimo:

- `contratos`: `id`, `cliente_id`, `inmueble_id`, `fecha_vencimiento_estimada`, `estado`, `created_at`, `updated_at`.
- `seguimientos`: `id`, `cliente_id`, `inmueble_id` opcional, `consulta_id` opcional, `agente_id`, `fecha_programada`, `motivo`, `estado`, `resultado`, `created_at`, `updated_at`.
- `notificaciones`: `id`, `cliente_id`, `tipo`, `canal`, `payload`, `idempotency_key UNIQUE`, `estado`, `provider_message_id`, `sent_at`, `error`.
- `auditoria`: `id`, `actor_id` o `actor_type`, `accion`, `entidad`, `entidad_id`, `antes`, `despues`, `request_id`, `created_at`.
- `consentimientos`: `id`, `cliente_id`, `canal`, `otorgado`, `fuente`, `fecha_otorgamiento`, `fecha_revocacion`.
- `favoritos`: `cliente_id`, `inmueble_id`, `created_at`, con clave única compuesta.

El historial de chat actual también es suficiente para una primera versión, pero una tabla `mensajes` será preferible si se necesitan búsquedas, retención, estados de entrega o auditoría por mensaje a gran escala.

## 13. Reglas de seguridad y operación

- Todo tráfico externo usa HTTPS/TLS.
- Las contraseñas se almacenan únicamente como hashes Argon2id o bcrypt con parámetros actualizados.
- Todas las consultas SQL se parametrizan; no se concatena entrada del usuario.
- Los endpoints públicos de consultas y webhooks aplican rate limiting, validación de tamaño y protección contra replay.
- Los registros con `deletedAt` se excluyen de consultas normales. La restauración requiere permiso administrativo.
- Los datos personales se minimizan en logs; nunca se registran contraseñas, tokens ni payloads completos de WhatsApp sin anonimización.
- Los cambios sobre clientes, inmuebles, consentimiento, sesiones y usuarios deben producir un evento de auditoría cuando exista la tabla correspondiente.
- Las operaciones compuestas sobre `personas` y `clientes` o `usuarios` se ejecutan en una transacción.
- Los jobs y envíos externos deben ser reintentables y utilizar claves de idempotencia.
- Los errores de proveedores externos se traducen a `502 UPSTREAM_ERROR` y se reintentan desde la cola, no desde el request del usuario.
- CORS debe restringirse a los dominios del frontend autorizado.
- La carga de imágenes debe validar MIME, tamaño, extensión, nombre y URL firmada del proveedor de almacenamiento.

### Observaciones del esquema actual

- `setup.sql` habilita `uuid-ossp`, pero usa `gen_random_uuid()`. En entornos donde esa función no sea parte del núcleo de PostgreSQL, debe habilitarse `pgcrypto` o cambiarse el default a `uuid_generate_v4()`.
- Las columnas `updated_at` tienen un valor inicial, pero no hay triggers para actualizarlas. La capa de servicio debe actualizarlas explícitamente o agregar triggers.
- `clientes` y `usuarios` heredan de `personas` por compartir su clave primaria; los repositorios deben consultar ambas tablas y aplicar el filtro de `personas.deleted_at`.
- `sesiones_chat.historial_mensajes` es JSONB. El servicio debe validar su esquema, limitar su tamaño y aplicar actualización optimista o bloqueo para evitar perder mensajes concurrentes.
- No hay una restricción SQL que garantice una sola portada o un solo teléfono principal. Esas reglas deben aplicarse en transacciones y, preferentemente, reforzarse con índices parciales.

## 14. Trazabilidad de requerimientos

| Requisito | Cobertura API |
|---|---|
| RF-01 a RF-06: CRUD, características, imágenes y búsqueda | `/inmuebles` y `/inmuebles/{id}/imagenes`. |
| RF-07 a RF-09: clientes, preferencias y búsquedas guardadas | `/clientes`, `/clientes/{id}/telefonos` y `/clientes/{id}/preferencias`. |
| RF-10: vencimiento de contratos | Pendiente: requiere `/contratos` y migración. |
| RF-11: agenda y recordatorios | Pendiente: requiere `/seguimientos` y `/notificaciones`. |
| RF-12: historial conversacional | `/sesiones-chat` y `/mensajes`. |
| RF-13 y RF-14: matching y recomendaciones | `/clientes/{id}/recomendaciones` y `/interno/jobs/matching`. |
| RF-15: consentimiento y opt-out | `/clientes/{id}/consentimiento`; historial completo pendiente. |
| RF-16: handoff humano | `/handoff`, `/tomar-control`, `/reanudar-ia` y `/cerrar`. |
| RF-17: usuarios, roles y permisos | `/auth` y `/usuarios`. |
| RF-18: auditoría | Requerida en todas las escrituras; endpoint y tabla pendientes. |
| RNF-01 a RNF-05: seguridad, RBAC y trazabilidad | Autenticación, scopes, validaciones, TLS y reglas de auditoría. |
| RNF-06 a RNF-08: crecimiento, recuperación e idempotencia | Paginación, workers, cola, backups operativos e `Idempotency-Key`. |
| RNF-10 y RNF-11: consentimiento y terceros | Bloqueo por consentimiento, adaptadores y validación de webhooks. |
