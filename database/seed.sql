-- ============================================================
-- Seed de desarrollo — Adelson
-- Combina el catálogo del sitio público (agente demo + 8 inmuebles)
-- con usuarios/CRM de prueba que tienen credenciales conocidas.
--
-- Requiere database/setup.sql y las migraciones aplicadas.
-- Es idempotente: puede ejecutarse varias veces (ON CONFLICT DO NOTHING).
--
-- Credenciales:
--   Administrador  admin@adelson.example   / Admin12345
--   Agente         carlos@adelson.example  / Agente12345
--   Agente demo    agente@adelson.example  / (aleatoria, no usable)
--   Clientes       (sin login propio)      / Cliente12345
--
-- Reset limpio opcional: descomentar el bloque TRUNCATE de abajo.
-- ============================================================
BEGIN;

-- Reset opcional (DESTRUCTIVO: borra todos los datos):
-- TRUNCATE sesiones_chat, consultas, preferencias_busquedas, imagenes,
--          inmuebles, clientes, usuarios, telefonos_persona, personas CASCADE;

-- ============================================================
-- 1) AGENTE DEMO + CATÁLOGO PÚBLICO (8 inmuebles)
-- ============================================================
INSERT INTO personas (id, nombre, apellido, dni, correo_electronico, contrasena_hash)
VALUES (
    '00000000-0000-4000-8000-000000000001',
    'Agente',
    'Demo',
    '30000000',
    'agente@adelson.example',
    crypt(gen_random_uuid()::text, gen_salt('bf'))
)
ON CONFLICT (correo_electronico) DO NOTHING;

INSERT INTO usuarios (id, rol)
SELECT id, 'Agente' FROM personas WHERE correo_electronico = 'agente@adelson.example'
ON CONFLICT (id) DO NOTHING;

INSERT INTO inmuebles (
    id, agente_id, titulo, descripcion, tipo_operacion, tipo_inmueble, estado,
    precio, moneda, monto_expensas, cant_ambientes, cant_habitaciones, cant_banos,
    cant_cocheras, superficie_total, superficie_cubierta, calle, numero, barrio,
    ciudad, latitud, longitud, caracteristicas
)
VALUES
(
    '11111111-1111-4111-8111-111111111101',
    (SELECT id FROM personas WHERE correo_electronico = 'agente@adelson.example'),
    'Casa moderna con pileta en Villa Morra',
    'Casa de cuatro habitaciones con pileta, quincho y cochera para dos vehículos.',
    'Venta', 'Casa', 'Disponible',
    185000, 'Dólares', 0, 5, 4, 3, 2, 320, 240,
    'Av. San Martín', '1450', 'Villa Morra', 'Resistencia', -27.4514, -58.9867,
    '{"piscina": true, "quincho": true, "amoblado": false}'::jsonb
),
(
    '11111111-1111-4111-8111-111111111102',
    (SELECT id FROM personas WHERE correo_electronico = 'agente@adelson.example'),
    'Departamento luminoso en el Centro',
    'Dos habitaciones, balcón al frente y cochera cubierta. Edificio con seguridad.',
    'Alquiler', 'Departamento', 'Disponible',
    450000, 'Pesos', 85000, 3, 2, 1, 1, 85, 70,
    'Junín', '820', 'Centro', 'Corrientes', -27.4692, -58.8306,
    '{"balcon": true, "seguridad": true, "amoblado": false}'::jsonb
),
(
    '11111111-1111-4111-8111-111111111103',
    (SELECT id FROM personas WHERE correo_electronico = 'agente@adelson.example'),
    'Terreno apto construcción en San Benito',
    'Lote de 500 m² con servicios de agua, luz y gas en la puerta.',
    'Venta', 'Terreno', 'Disponible',
    60000, 'Dólares', 0, NULL, NULL, NULL, NULL, 500, NULL,
    'Ruta 11', 'Km 1004', 'San Benito', 'Resistencia', -27.4833, -58.9500,
    '{"agua": true, "luz": true, "gas": true}'::jsonb
),
(
    '11111111-1111-4111-8111-111111111104',
    (SELECT id FROM personas WHERE correo_electronico = 'agente@adelson.example'),
    'Local comercial a la calle en el Centro',
    'Local de 150 m² con vidriera, baño y depósito. Ideal para showroom.',
    'Alquiler', 'Local Comercial', 'Disponible',
    800000, 'Pesos', 50000, 2, NULL, 1, NULL, 150, 150,
    'Mitre', '340', 'Centro', 'Resistencia', -27.4500, -58.9900,
    '{"vidriera": true, "deposito": true}'::jsonb
),
(
    '11111111-1111-4111-8111-111111111105',
    (SELECT id FROM personas WHERE correo_electronico = 'agente@adelson.example'),
    'Galpón industrial con oficinas',
    'Galpón de 600 m² cubiertos con oficinas, baños y portón para camión.',
    'Alquiler', 'Galpón', 'Disponible',
    1200000, 'Pesos', 0, 3, NULL, 2, 4, 600, 600,
    'Av. Paraguay', '2500', 'Parque Industrial', 'Resistencia', -27.4300, -58.9600,
    '{"oficinas": true, "porton_camion": true, "trifasico": true}'::jsonb
),
(
    '11111111-1111-4111-8111-111111111106',
    (SELECT id FROM personas WHERE correo_electronico = 'agente@adelson.example'),
    'Campo de 15 hectáreas con acceso pavimentado',
    'Campo con alambrado perimetral, molino y acceso directo desde ruta pavimentada.',
    'Venta', 'Campo', 'Disponible',
    450000, 'Dólares', 0, NULL, NULL, NULL, NULL, 150000, NULL,
    'Ruta 16', 'Km 25', 'Zona Rural', 'Resistencia', -27.3500, -59.0500,
    '{"molino": true, "alambrado": true, "acceso_pavimentado": true}'::jsonb
),
(
    '11111111-1111-4111-8111-111111111107',
    (SELECT id FROM personas WHERE correo_electronico = 'agente@adelson.example'),
    'Casa a metros de la playa',
    'Casa de tres habitaciones para alquiler temporario, con patio y parrilla.',
    'Alquiler Temporario', 'Casa', 'Disponible',
    90000, 'Pesos', 0, 4, 3, 2, 1, 120, 100,
    'Costanera', '120', 'Playa', 'Corrientes', -27.4600, -58.8400,
    '{"parrilla": true, "patio": true, "amoblado": true}'::jsonb
),
(
    '11111111-1111-4111-8111-111111111108',
    (SELECT id FROM personas WHERE correo_electronico = 'agente@adelson.example'),
    'Departamento premium en Barrio Norte',
    'Tres habitaciones, dos baños, cochera y amenities completos.',
    'Venta', 'Departamento', 'Disponible',
    120000000, 'Pesos', 120000, 4, 3, 2, 2, 140, 120,
    'Av. Lavalle', '2100', 'Barrio Norte', 'Resistencia', -27.4400, -58.9800,
    '{"piscina": true, "gimnasio": true, "seguridad": true}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO imagenes (id, inmueble_id, url_archivo, orden_visualizacion, es_portada)
VALUES
('22222222-2222-4222-8222-222222222101', '11111111-1111-4111-8111-111111111101', 'https://images.unsplash.com/photo-1600585154340-be6161a56a0c?auto=format&fit=crop&w=1600&q=70', 1, true),
('22222222-2222-4222-8222-222222222102', '11111111-1111-4111-8111-111111111102', 'https://images.unsplash.com/photo-1522708323590-d24dbb6b0267?auto=format&fit=crop&w=1600&q=70', 1, true),
('22222222-2222-4222-8222-222222222103', '11111111-1111-4111-8111-111111111103', 'https://images.unsplash.com/photo-1500382017468-9049fed747ef?auto=format&fit=crop&w=1600&q=70', 1, true),
('22222222-2222-4222-8222-222222222104', '11111111-1111-4111-8111-111111111104', 'https://images.unsplash.com/photo-1524758631624-e2822e304c36?auto=format&fit=crop&w=1600&q=70', 1, true),
('22222222-2222-4222-8222-222222222105', '11111111-1111-4111-8111-111111111105', 'https://images.unsplash.com/photo-1581094794329-c8112a89af12?auto=format&fit=crop&w=1600&q=70', 1, true),
('22222222-2222-4222-8222-222222222106', '11111111-1111-4111-8111-111111111106', 'https://images.unsplash.com/photo-1501785888041-af3ef285b470?auto=format&fit=crop&w=1600&q=70', 1, true),
('22222222-2222-4222-8222-222222222107', '11111111-1111-4111-8111-111111111107', 'https://images.unsplash.com/photo-1564013799919-ab600027ffc6?auto=format&fit=crop&w=1600&q=70', 1, true),
('22222222-2222-4222-8222-222222222108', '11111111-1111-4111-8111-111111111108', 'https://images.unsplash.com/photo-1493809842364-78817add7ffb?auto=format&fit=crop&w=1600&q=70', 1, true)
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 2) USUARIOS INTERNOS DE PRUEBA (login conocido)
-- ============================================================
-- Administrador: admin@adelson.example / Admin12345
INSERT INTO personas (id, nombre, apellido, dni, correo_electronico, contrasena_hash) VALUES
('10000000-0000-0000-0000-000000000001', 'Ana',  'Pérez', '20111222',
 'admin@adelson.example',
 '$2a$10$0twY3UDEYJjLLkGptWbYYOC6y95bf/ddp.UWB3OvoIqZSETpvYFhG')
ON CONFLICT DO NOTHING;

INSERT INTO usuarios (id, rol, estado_operativo) VALUES
('10000000-0000-0000-0000-000000000001', 'Administrador', 'Activo')
ON CONFLICT (id) DO NOTHING;

-- Agente: carlos@adelson.example / Agente12345
INSERT INTO personas (id, nombre, apellido, dni, correo_electronico, contrasena_hash) VALUES
('10000000-0000-0000-0000-000000000002', 'Carlos', 'López', '25123456',
 'carlos@adelson.example',
 '$2a$10$lpxen9smIx8/uBH.HLemKO/wn4khGdW0G5MbJ7/9j.5dLS96Jpgg2')
ON CONFLICT DO NOTHING;

INSERT INTO usuarios (id, rol, estado_operativo) VALUES
('10000000-0000-0000-0000-000000000002', 'Agente', 'Activo')
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 3) CLIENTES + TELÉFONOS
-- ============================================================
INSERT INTO personas (id, nombre, apellido, dni, correo_electronico, contrasena_hash) VALUES
('20000000-0000-0000-0000-000000000001', 'María',  'Gómez',     '32999888', 'maria@example.com',  '$2a$10$.tiPDO2gusHsHMOGbkhrbOlnenFNH736k8rYhYsTJq2IAAku4WJGe'),
('20000000-0000-0000-0000-000000000002', 'Juan',   'Fernández', '30111222', 'juan@example.com',   '$2a$10$.tiPDO2gusHsHMOGbkhrbOlnenFNH736k8rYhYsTJq2IAAku4WJGe'),
('20000000-0000-0000-0000-000000000003', 'Lucía',  'Rodríguez', '27888999', 'lucia@example.com',  '$2a$10$.tiPDO2gusHsHMOGbkhrbOlnenFNH736k8rYhYsTJq2IAAku4WJGe')
ON CONFLICT DO NOTHING;

INSERT INTO clientes (id, consentimiento_notificaciones, estado_lead, tipo_cliente_principal) VALUES
('20000000-0000-0000-0000-000000000001', true,  'Contactado',      'Buscando Alquiler'),
('20000000-0000-0000-0000-000000000002', true,  'Nuevo',           'Buscando Comprar'),
('20000000-0000-0000-0000-000000000003', false, 'En Negociación',  'Propietario')
ON CONFLICT (id) DO NOTHING;

INSERT INTO telefonos_persona (id, persona_id, numero, es_principal) VALUES
('50000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001', '+5493492123456', true),
('50000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000002', '+5493492654321', true),
('50000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000003', '+5493492789456', true)
ON CONFLICT DO NOTHING;

-- ============================================================
-- 4) INMUEBLES DE PRUEBA (agente_id = Carlos)
-- ============================================================
INSERT INTO inmuebles (id, agente_id, titulo, descripcion, tipo_operacion, tipo_inmueble, estado,
                       precio, moneda, monto_expensas, cant_ambientes, cant_habitaciones, cant_banos,
                       cant_cocheras, superficie_total, superficie_cubierta, calle, numero, barrio,
                       ciudad, latitud, longitud, caracteristicas) VALUES
('30000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000002',
 'Departamento luminoso en el Centro', 'Dos habitaciones, balcón y cochera.', 'Alquiler', 'Departamento',
 'Disponible', 800.00, 'Dólares', 95000.00, 3, 2, 1, 1, 70.00, 65.00, 'San Martín', '123', 'Centro',
 'Rafaela', -31.25000000, -61.48670000, '{"balcon":true,"aptoMascotas":true,"calefaccion":"Gas"}'),

('30000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000002',
 'Casa familiar con patio', 'Casa de 4 ambientes con quincho.', 'Venta', 'Casa', 'Disponible',
 185000.00, 'Dólares', 0, 4, 3, 2, 2, 180.00, 160.00, '9 de Julio', '455', 'Barrio Norte',
 'Rafaela', -31.24000000, -61.49000000, '{"patio":true,"parrilla":true,"cochera":2}'),

('30000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000002',
 'Departamento 2 ambientes', 'Ideal para pareja, cerca de plaza.', 'Alquiler', 'Departamento',
 'Disponible', 350000.00, 'Pesos', 48000.00, 2, 1, 1, 0, 45.00, 42.00, 'Belgrano', '200', 'Centro',
 'Rafaela', -31.25200000, -61.48800000, '{"balcon":false,"aptoMascotas":false}'),

('30000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000002',
 'Terreno en zona de crecimiento', 'Lote apto para vivienda o comercio.', 'Venta', 'Terreno',
 'Disponible', 25000000.00, 'Pesos', 0, NULL, NULL, NULL, NULL, 400.00, NULL, NULL, NULL, 'Zona Sur',
 'Rafaela', -31.26000000, -61.47500000, '{"servicios":"agua, luz"}'),

('30000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000002',
 'Local comercial céntrico', 'Frente vidriado sobre peatonal.', 'Alquiler', 'Local Comercial',
 'Reservado', 900000.00, 'Pesos', 120000.00, 1, NULL, 1, 0, 85.00, 80.00, '25 de Mayo', '98', 'Centro',
 'Rafaela', -31.24900000, -61.48700000, '{"vidriera":true,"deposito":true}')
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 5) IMÁGENES DE LOS INMUEBLES DE PRUEBA
-- ============================================================
INSERT INTO imagenes (id, inmueble_id, url_archivo, orden_visualizacion, es_portada) VALUES
('40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001',
 'https://cdn.adelson.example/inmuebles/dpto-centro/frente.webp', 1, true),
('40000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001',
 'https://cdn.adelson.example/inmuebles/dpto-centro/balcon.webp', 2, false),
('40000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000002',
 'https://cdn.adelson.example/inmuebles/casa-patio/frente.webp', 1, true)
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 6) PREFERENCIAS DE BÚSQUEDA
-- ============================================================
INSERT INTO preferencias_busquedas (id, cliente_id, rango_precio_min, rango_precio_max, moneda,
                                    tipo_operacion, tipo_inmueble, cantidad_dormitorios_min, zona_preferida) VALUES
('60000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001',
 500.00, 800.00, 'Dólares', 'Alquiler', 'Departamento', 2, 'Centro'),
('60000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000002',
 150000.00, 200000.00, 'Dólares', 'Venta', 'Casa', 3, NULL)
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 7) CONSULTAS
-- ============================================================
INSERT INTO consultas (id, cliente_id, inmueble_id, canal_origen, estado_seguimiento, notas_internas) VALUES
('70000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001',
 '30000000-0000-0000-0000-000000000001', 'Portal Web', 'Visita Agendada', 'Visita confirmada para el viernes a las 17:00.'),
('70000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000002',
 '30000000-0000-0000-0000-000000000002', 'WhatsApp', 'Pendiente', NULL),
('70000000-0000-0000-0000-000000000003', '20000000-0000-0000-0000-000000000003',
 '30000000-0000-0000-0000-000000000005', 'Manual', 'Contactado', NULL)
ON CONFLICT (id) DO NOTHING;

-- ============================================================
-- 8) SESIONES DE CHAT
-- ============================================================
INSERT INTO sesiones_chat (id, cliente_id, agente_asignado_id, estado_sesion, historial_mensajes) VALUES
('80000000-0000-0000-0000-000000000001', '20000000-0000-0000-0000-000000000001',
 NULL, 'Atendida por IA',
 '[
   {"id":"b1","remitenteTipo":"cliente","contenido":"Hola, busco un depto de 2 ambientes","fechaHora":"2026-10-10T14:00:00Z"},
   {"id":"b2","remitenteTipo":"ia","contenido":"Encontré propiedades que coinciden","fechaHora":"2026-10-10T14:00:05Z","metadata":{"modelo":"adelai","tool":"buscar_propiedades"}}
 ]'::jsonb),
('80000000-0000-0000-0000-000000000002', '20000000-0000-0000-0000-000000000002',
 '10000000-0000-0000-0000-000000000002', 'Esperando Humano', '[]'::jsonb)
ON CONFLICT (id) DO NOTHING;

COMMIT;
