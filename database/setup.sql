-- Habilitar UUIDs (por si usas una versión antigua, aunque Neon lo trae por defecto)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ENUMs para el Módulo de Personas y CRM
CREATE TYPE estado_lead_enum AS ENUM ('Nuevo', 'Contactado', 'En Negociación', 'Cerrado', 'Inactivo');
CREATE TYPE tipo_cliente_enum AS ENUM ('Buscando Alquiler', 'Buscando Comprar', 'Propietario');
CREATE TYPE rol_usuario_enum AS ENUM ('Administrador', 'Agente');
CREATE TYPE estado_operativo_enum AS ENUM ('Activo', 'Inactivo', 'Suspendido');
CREATE TYPE canal_origen_enum AS ENUM ('WhatsApp', 'Portal Web', 'Manual');
CREATE TYPE estado_seguimiento_enum AS ENUM ('Pendiente', 'Contactado', 'Visita Agendada', 'Descartado', 'Cerrado');
CREATE TYPE estado_sesion_enum AS ENUM ('Atendida por IA', 'Esperando Humano', 'Atendida por Humano', 'Cerrada');

-- ENUMs para el Catálogo de Inmuebles
CREATE TYPE tipo_operacion_enum AS ENUM ('Venta', 'Alquiler', 'Alquiler Temporario');
CREATE TYPE tipo_inmueble_enum AS ENUM ('Casa', 'Departamento', 'Terreno', 'Local Comercial', 'Galpón');
CREATE TYPE estado_inmueble_enum AS ENUM ('Disponible', 'Reservado', 'Vendido', 'Alquilado', 'Oculto');
CREATE TYPE moneda_enum AS ENUM ('Pesos', 'Dólares');


-- SUPERCLASE: PERSONA
CREATE TABLE personas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre VARCHAR(100) NOT NULL,
    apellido VARCHAR(100) NOT NULL,
    dni VARCHAR(20) UNIQUE NOT NULL,
    correo_electronico VARCHAR(255) UNIQUE NOT NULL,
    contrasena_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP -- Soft delete
);

-- Atributo Multivaluado (Teléfonos)
CREATE TABLE telefonos_persona (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    persona_id UUID NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    numero VARCHAR(20) NOT NULL,
    es_principal BOOLEAN DEFAULT false,
    UNIQUE(persona_id, numero)
);

-- SUBCLASE: CLIENTE
CREATE TABLE clientes (
    id UUID PRIMARY KEY REFERENCES personas(id) ON DELETE CASCADE,
    consentimiento_notificaciones BOOLEAN DEFAULT false,
    estado_lead estado_lead_enum DEFAULT 'Nuevo',
    tipo_cliente_principal tipo_cliente_enum,
    fecha_registro TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    fecha_ultima_interaccion TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- SUBCLASE: USUARIO (Admin/Agente)
CREATE TABLE usuarios (
    id UUID PRIMARY KEY REFERENCES personas(id) ON DELETE CASCADE,
    rol rol_usuario_enum NOT NULL,
    estado_operativo estado_operativo_enum DEFAULT 'Activo',
    fecha_alta TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ultimo_acceso TIMESTAMP
);




CREATE TABLE inmuebles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agente_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE RESTRICT, -- El Agente que lo gestiona
    
    -- Datos Comerciales
    titulo VARCHAR(150) NOT NULL,
    descripcion TEXT,
    tipo_operacion tipo_operacion_enum NOT NULL,
    tipo_inmueble tipo_inmueble_enum NOT NULL,
    estado estado_inmueble_enum DEFAULT 'Disponible',
    precio NUMERIC(15, 2) NOT NULL,
    moneda moneda_enum NOT NULL,
    monto_expensas NUMERIC(15, 2) DEFAULT 0,
    
    -- Dimensiones
    cant_ambientes INT,
    cant_habitaciones INT,
    cant_banos INT,
    cant_cocheras INT,
    superficie_total NUMERIC(10, 2),
    superficie_cubierta NUMERIC(10, 2),
    
    -- Ubicación (Atributo Compuesto aplanado)
    calle VARCHAR(150),
    numero VARCHAR(20),
    barrio VARCHAR(100),
    ciudad VARCHAR(100),
    latitud NUMERIC(10, 8),
    longitud NUMERIC(11, 8),
    
    -- Características Dinámicas
    caracteristicas JSONB DEFAULT '{}'::jsonb,
    
    -- Auditoría
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE TABLE imagenes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    inmueble_id UUID NOT NULL REFERENCES inmuebles(id) ON DELETE CASCADE,
    url_archivo TEXT NOT NULL,
    orden_visualizacion INT DEFAULT 1,
    es_portada BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);



-- PREFERENCIAS DE BÚSQUEDA (1 a N con Cliente)
CREATE TABLE preferencias_busquedas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cliente_id UUID NOT NULL REFERENCES clientes(id) ON DELETE CASCADE,
    rango_precio_min NUMERIC(15, 2),
    rango_precio_max NUMERIC(15, 2),
    moneda moneda_enum,
    tipo_operacion tipo_operacion_enum,
    tipo_inmueble tipo_inmueble_enum,
    cantidad_dormitorios_min INT,
    zona_preferida VARCHAR(100),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- CONSULTAS (Entidad Asociativa / Tabla Puente M:N)
CREATE TABLE consultas (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cliente_id UUID NOT NULL REFERENCES clientes(id) ON DELETE CASCADE,
    inmueble_id UUID NOT NULL REFERENCES inmuebles(id) ON DELETE CASCADE,
    fecha_hora TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    canal_origen canal_origen_enum NOT NULL,
    estado_seguimiento estado_seguimiento_enum DEFAULT 'Pendiente',
    notas_internas TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- SESIONES DE CHAT (AdelAI + Handoff Humano)
CREATE TABLE sesiones_chat (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cliente_id UUID NOT NULL REFERENCES clientes(id) ON DELETE CASCADE,
    
    -- ¡Aquí está la relación con el Agente! (0..1 a N)
    -- Es NULL mientras la IA atiende, se llena con el ID del usuario cuando hay intervención humana.
    agente_asignado_id UUID REFERENCES usuarios(id) ON DELETE SET NULL, 
    
    fecha_inicio TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    fecha_ultima_interaccion TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    estado_sesion estado_sesion_enum DEFAULT 'Atendida por IA',
    
    -- El array de mensajes se guarda aquí de forma estructurada
    historial_mensajes JSONB DEFAULT '[]'::jsonb
);
