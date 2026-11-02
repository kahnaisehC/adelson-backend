-- Datos de ejemplo para el sitio público.
-- Requiere que database/setup.sql ya esté aplicado.
-- Reemplazar pub-xxxxxxxx.r2.dev por el dominio público real del bucket R2.

INSERT INTO personas (id, nombre, apellido, dni, correo_electronico, contrasena_hash)
VALUES (
    '00000000-0000-4000-8000-000000000001',
    'Agente',
    'Demo',
    '30000000',
    'agente@adelson.example',
    crypt('Adelson123!', gen_salt('bf'))
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
('22222222-2222-4222-8222-222222222101', '11111111-1111-4111-8111-111111111101', 'https://pub-xxxxxxxx.r2.dev/inmuebles/casa-villa-morra.jpg', 1, true),
('22222222-2222-4222-8222-222222222102', '11111111-1111-4111-8111-111111111102', 'https://pub-xxxxxxxx.r2.dev/inmuebles/departamento-centro.jpg', 1, true),
('22222222-2222-4222-8222-222222222103', '11111111-1111-4111-8111-111111111103', 'https://pub-xxxxxxxx.r2.dev/inmuebles/terreno-san-benito.jpg', 1, true),
('22222222-2222-4222-8222-222222222104', '11111111-1111-4111-8111-111111111104', 'https://pub-xxxxxxxx.r2.dev/inmuebles/local-centro.jpg', 1, true),
('22222222-2222-4222-8222-222222222105', '11111111-1111-4111-8111-111111111105', 'https://pub-xxxxxxxx.r2.dev/inmuebles/galpon-industrial.jpg', 1, true),
('22222222-2222-4222-8222-222222222106', '11111111-1111-4111-8111-111111111106', 'https://pub-xxxxxxxx.r2.dev/inmuebles/campo-ruta-16.jpg', 1, true),
('22222222-2222-4222-8222-222222222107', '11111111-1111-4111-8111-111111111107', 'https://pub-xxxxxxxx.r2.dev/inmuebles/casa-playa.jpg', 1, true),
('22222222-2222-4222-8222-222222222108', '11111111-1111-4111-8111-111111111108', 'https://pub-xxxxxxxx.r2.dev/inmuebles/departamento-barrio-norte.jpg', 1, true)
ON CONFLICT (id) DO NOTHING;
