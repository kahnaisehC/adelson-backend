-- Panel administrativo: sesiones de refresh, asignación de consultas e imágenes en R2.

ALTER TABLE consultas
    ADD COLUMN IF NOT EXISTS agente_asignado_id UUID REFERENCES usuarios(id) ON DELETE SET NULL;

ALTER TABLE imagenes
    ADD COLUMN IF NOT EXISTS storage_key TEXT;

CREATE TABLE IF NOT EXISTS sesiones_refresco (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    usuario_id UUID NOT NULL REFERENCES usuarios(id) ON DELETE CASCADE,
    jti UUID NOT NULL UNIQUE,
    user_agent TEXT,
    ip VARCHAR(64),
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sesiones_refresco_usuario ON sesiones_refresco(usuario_id);
CREATE INDEX IF NOT EXISTS idx_consultas_agente ON consultas(agente_asignado_id);
CREATE INDEX IF NOT EXISTS idx_consultas_estado ON consultas(estado_seguimiento);
CREATE INDEX IF NOT EXISTS idx_consultas_fecha ON consultas(fecha_hora DESC);
