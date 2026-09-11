**Documentación de Base de Datos: Plataforma Inmobiliaria y CRM**

Este diseño relacional soporta el catálogo de propiedades, el embudo de ventas y la integración directa con el asistente virtual AdelAI, asegurando consistencia e integridad referencial.

**Diccionario de Tablas Principales**

| **Tabla Relacional**   | **Módulo del Sistema** | **Propósito Operativo**                                                               |
|------------------------|------------------------|---------------------------------------------------------------------------------------|
| personas               | Seguridad / Core       | Centraliza las credenciales, UUIDs y autenticación de todos los actores.              |
| clientes               | CRM / Ventas           | Almacena métricas de adquisición y segmentación de prospectos (leads).                |
| usuarios               | Administración         | Define los roles operativos, estado y accesos del personal interno.                   |
| telefonos_persona      | Seguridad / Core       | Resuelve la restricción de normalización aislando los contactos múltiples.            |
| inmuebles              | Catálogo Web           | Contiene datos comerciales, espaciales y un esquema flexible para amenidades.         |
| imagenes               | Catálogo Web           | Indexa las referencias de almacenamiento externo vinculadas a una propiedad.          |
| preferencias_busquedas | CRM / Ventas           | Cuantifica los criterios ideales de un prospecto para el emparejamiento automático.   |
| consultas              | CRM / Ventas           | Registra el instante exacto en que un interesado interactúa con una publicación.      |
| sesiones_chat          | Asistente IA           | Mantiene el contexto de las conversaciones de WhatsApp y gestiona el traspaso manual. |

**Decisiones Arquitectónicas Aplicadas**

- Implementación de UUIDs en todas las claves primarias para proteger los puntos de acceso de la API en Go.

- Uso estricto del patrón de herencia basada en tablas para modelar la separación de perfiles.

- Inclusión de campos de auditoría temporales en las entidades principales del esquema.

- Aplicación obligatoria de borrado lógico (Soft Delete) para preservar el historial de análisis comercial.

- Resolución de atributos multivaluados mediante tablas satélites exclusivas.

- Adopción de columnas de almacenamiento flexible para características inmobiliarias dinámicas.

- Consolidación del historial de mensajería en estructuras unificadas de lectura rápida para agilizar el procesamiento del LLM.

- Diseño de estados relacionales optimizados para coordinar el tráfico entre respuestas automatizadas y la intervención humana.
