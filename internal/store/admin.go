package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type AuthUser struct {
	ID          string
	FirstName   string
	LastName    string
	Email       string
	Hash        string
	Role        string
	Operational string
}

func (s *Store) GetAuthUserByEmail(ctx context.Context, email string) (AuthUser, error) {
	if s.DB == nil {
		return AuthUser{}, errors.New("database is not configured")
	}
	var u AuthUser
	err := s.DB.QueryRowContext(ctx, `SELECT p.id, p.nombre, p.apellido, p.correo_electronico, p.contrasena_hash, u.rol, u.estado_operativo FROM personas p JOIN usuarios u ON u.id = p.id WHERE lower(p.correo_electronico) = lower($1) AND p.deleted_at IS NULL`, email).Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Hash, &u.Role, &u.Operational)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthUser{}, ErrNotFound
	}
	return u, err
}

func (s *Store) GetAuthUser(ctx context.Context, id string) (AuthUser, error) {
	if s.DB == nil {
		return AuthUser{}, errors.New("database is not configured")
	}
	var u AuthUser
	err := s.DB.QueryRowContext(ctx, `SELECT p.id, p.nombre, p.apellido, p.correo_electronico, p.contrasena_hash, u.rol, u.estado_operativo FROM personas p JOIN usuarios u ON u.id = p.id WHERE p.id = $1 AND p.deleted_at IS NULL`, id).Scan(&u.ID, &u.FirstName, &u.LastName, &u.Email, &u.Hash, &u.Role, &u.Operational)
	if errors.Is(err, sql.ErrNoRows) {
		return AuthUser{}, ErrNotFound
	}
	return u, err
}

func (s *Store) UpdatePassword(ctx context.Context, id, hash string) error {
	if s.DB == nil {
		return errors.New("database is not configured")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE personas SET contrasena_hash = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND deleted_at IS NULL`, id, hash)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CountActiveAdmins(ctx context.Context) (int, error) {
	if s.DB == nil {
		return 0, errors.New("database is not configured")
	}
	var count int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usuarios u JOIN personas p ON p.id = u.id WHERE u.rol = 'Administrador' AND u.estado_operativo = 'Activo' AND p.deleted_at IS NULL`).Scan(&count)
	return count, err
}

func (s *Store) CreateRefreshSession(ctx context.Context, userID, jti, userAgent, ip string, expires time.Time) error {
	if s.DB == nil {
		return errors.New("database is not configured")
	}
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sesiones_refresco (usuario_id, jti, user_agent, ip, expires_at) VALUES ($1, $2, $3, $4, $5)`, userID, jti, userAgent, ip, expires)
	return err
}

func (s *Store) RotateRefreshSession(ctx context.Context, oldJTI, newJTI, userID, userAgent, ip string, expires time.Time) error {
	if s.DB == nil {
		return errors.New("database is not configured")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE sesiones_refresco SET revoked_at = CURRENT_TIMESTAMP WHERE jti = $1 AND usuario_id = $2 AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP`, oldJTI, userID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sesiones_refresco (usuario_id, jti, user_agent, ip, expires_at) VALUES ($1, $2, $3, $4, $5)`, userID, newJTI, userAgent, ip, expires); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) RefreshSessionValid(ctx context.Context, userID, jti string) (bool, error) {
	if s.DB == nil {
		return false, errors.New("database is not configured")
	}
	var exists bool
	err := s.DB.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sesiones_refresco WHERE usuario_id = $1 AND jti = $2 AND revoked_at IS NULL AND expires_at > CURRENT_TIMESTAMP)`, userID, jti).Scan(&exists)
	return exists, err
}

func (s *Store) RevokeRefreshSession(ctx context.Context, jti string) error {
	if s.DB == nil {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE sesiones_refresco SET revoked_at = CURRENT_TIMESTAMP WHERE jti = $1 AND revoked_at IS NULL`, jti)
	return err
}

func (s *Store) RevokeUserSessions(ctx context.Context, userID string) error {
	if s.DB == nil {
		return nil
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE sesiones_refresco SET revoked_at = CURRENT_TIMESTAMP WHERE usuario_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (s *Store) PropertyOwner(ctx context.Context, id string) (string, error) {
	if s.DB == nil {
		return "", errors.New("database is not configured")
	}
	var owner string
	err := s.DB.QueryRowContext(ctx, `SELECT agente_id FROM inmuebles WHERE id = $1`, id).Scan(&owner)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return owner, err
}

func (s *Store) RestoreProperty(ctx context.Context, id string) (Property, error) {
	if s.DB == nil {
		return Property{}, errors.New("database is not configured")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE inmuebles SET deleted_at = NULL, estado = 'Disponible', updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return Property{}, fmt.Errorf("restore property: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Property{}, ErrNotFound
	}
	return s.GetProperty(ctx, id, true)
}

func (s *Store) GetImage(ctx context.Context, propertyID, imageID string) (Image, error) {
	if s.DB == nil {
		return Image{}, errors.New("database is not configured")
	}
	var image Image
	var storageKey sql.NullString
	err := s.DB.QueryRowContext(ctx, `SELECT id, inmueble_id, url_archivo, storage_key, orden_visualizacion, es_portada, created_at FROM imagenes WHERE id = $1 AND inmueble_id = $2`, imageID, propertyID).Scan(&image.ID, &image.PropertyID, &image.FileURL, &storageKey, &image.DisplayOrder, &image.IsCover, &image.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, ErrNotFound
	}
	image.StorageKey = stringPtr(storageKey)
	return image, err
}

func (s *Store) UpdateImage(ctx context.Context, propertyID, imageID string, order *int, cover *bool) (Image, error) {
	if s.DB == nil {
		return Image{}, errors.New("database is not configured")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Image{}, err
	}
	defer tx.Rollback()
	if cover != nil && *cover {
		if _, err := tx.ExecContext(ctx, `UPDATE imagenes SET es_portada = false WHERE inmueble_id = $1`, propertyID); err != nil {
			return Image{}, err
		}
	}
	result, err := tx.ExecContext(ctx, `UPDATE imagenes SET orden_visualizacion = COALESCE($3, orden_visualizacion), es_portada = COALESCE($4, es_portada) WHERE id = $1 AND inmueble_id = $2`, imageID, propertyID, order, cover)
	if err != nil {
		return Image{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Image{}, ErrNotFound
	}
	if err := tx.Commit(); err != nil {
		return Image{}, err
	}
	return s.GetImage(ctx, propertyID, imageID)
}

func (s *Store) DeleteImage(ctx context.Context, propertyID, imageID string) (Image, error) {
	image, err := s.GetImage(ctx, propertyID, imageID)
	if err != nil {
		return Image{}, err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM imagenes WHERE id = $1 AND inmueble_id = $2`, imageID, propertyID); err != nil {
		return Image{}, err
	}
	if image.IsCover {
		_, _ = s.DB.ExecContext(ctx, `UPDATE imagenes SET es_portada = true WHERE id = (SELECT id FROM imagenes WHERE inmueble_id = $1 ORDER BY orden_visualizacion, created_at LIMIT 1)`, propertyID)
	}
	return image, nil
}

type InquiryDetail struct {
	ID             string    `json:"id"`
	ClientID       string    `json:"clienteId"`
	ClientName     string    `json:"clienteNombre"`
	ClientEmail    string    `json:"clienteCorreoElectronico"`
	ClientPhone    *string   `json:"clienteTelefono,omitempty"`
	PropertyID     *string   `json:"inmuebleId,omitempty"`
	PropertyTitle  *string   `json:"inmuebleTitulo,omitempty"`
	AssignedAgent  *string   `json:"agenteAsignadoId,omitempty"`
	AgentName      *string   `json:"agenteNombre,omitempty"`
	DateTime       time.Time `json:"fechaHora"`
	OriginChannel  string    `json:"canalOrigen"`
	FollowUpStatus string    `json:"estadoSeguimiento"`
	InternalNotes  *string   `json:"notasInternas,omitempty"`
	Message        *string   `json:"mensaje,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type InquiryFilter struct {
	Status   string
	Channel  string
	AgentID  string
	Query    string
	Page     int
	PageSize int
}

func (s *Store) ListInquiryDetails(ctx context.Context, filter InquiryFilter, restrictAgent string) ([]InquiryDetail, int, error) {
	if s.DB == nil {
		return nil, 0, errors.New("database is not configured")
	}
	where := []string{"1 = 1"}
	args := []interface{}{}
	if filter.Status != "" {
		args = append(args, filter.Status)
		where = append(where, fmt.Sprintf("c.estado_seguimiento = $%d", len(args)))
	}
	if filter.Channel != "" {
		args = append(args, filter.Channel)
		where = append(where, fmt.Sprintf("c.canal_origen = $%d", len(args)))
	}
	if filter.AgentID != "" {
		args = append(args, filter.AgentID)
		where = append(where, fmt.Sprintf("c.agente_asignado_id = $%d", len(args)))
	}
	if restrictAgent != "" {
		args = append(args, restrictAgent)
		where = append(where, fmt.Sprintf("c.agente_asignado_id = $%d", len(args)))
	}
	if filter.Query != "" {
		args = append(args, "%"+strings.ToLower(filter.Query)+"%")
		where = append(where, fmt.Sprintf("(lower(p.nombre || ' ' || p.apellido) LIKE $%d OR lower(p.correo_electronico) LIKE $%d OR lower(COALESCE(i.titulo, '')) LIKE $%d)", len(args), len(args), len(args)))
	}
	clause := strings.Join(where, " AND ")
	var total int
	countQuery := `SELECT COUNT(*) FROM consultas c JOIN personas p ON p.id = c.cliente_id LEFT JOIN inmuebles i ON i.id = c.inmueble_id WHERE ` + clause
	if err := s.DB.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	query := `SELECT c.id, c.cliente_id, p.nombre || ' ' || p.apellido, p.correo_electronico,
		(SELECT t.numero FROM telefonos_persona t WHERE t.persona_id = p.id ORDER BY t.es_principal DESC, t.id LIMIT 1),
		c.inmueble_id, i.titulo, c.agente_asignado_id,
		(SELECT a.nombre || ' ' || a.apellido FROM personas a WHERE a.id = c.agente_asignado_id),
		c.fecha_hora, c.canal_origen, c.estado_seguimiento, c.notas_internas, c.updated_at
		FROM consultas c JOIN personas p ON p.id = c.cliente_id LEFT JOIN inmuebles i ON i.id = c.inmueble_id
		WHERE ` + clause + ` ORDER BY c.fecha_hora DESC LIMIT $` + fmt.Sprint(len(args)-1) + ` OFFSET $` + fmt.Sprint(len(args))
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []InquiryDetail{}
	for rows.Next() {
		item, err := scanInquiryDetail(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, item)
	}
	return result, total, rows.Err()
}

func (s *Store) GetInquiryDetail(ctx context.Context, id string) (InquiryDetail, error) {
	if s.DB == nil {
		return InquiryDetail{}, errors.New("database is not configured")
	}
	row := s.DB.QueryRowContext(ctx, `SELECT c.id, c.cliente_id, p.nombre || ' ' || p.apellido, p.correo_electronico,
		(SELECT t.numero FROM telefonos_persona t WHERE t.persona_id = p.id ORDER BY t.es_principal DESC, t.id LIMIT 1),
		c.inmueble_id, i.titulo, c.agente_asignado_id,
		(SELECT a.nombre || ' ' || a.apellido FROM personas a WHERE a.id = c.agente_asignado_id),
		c.fecha_hora, c.canal_origen, c.estado_seguimiento, c.notas_internas, c.updated_at
		FROM consultas c JOIN personas p ON p.id = c.cliente_id LEFT JOIN inmuebles i ON i.id = c.inmueble_id WHERE c.id = $1`, id)
	item, err := scanInquiryDetail(row)
	if errors.Is(err, sql.ErrNoRows) {
		return InquiryDetail{}, ErrNotFound
	}
	return item, err
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanInquiryDetail(row rowScanner) (InquiryDetail, error) {
	var item InquiryDetail
	var phone, propertyID, propertyTitle, agentID, agentName, notes sql.NullString
	err := row.Scan(&item.ID, &item.ClientID, &item.ClientName, &item.ClientEmail, &phone, &propertyID, &propertyTitle, &agentID, &agentName, &item.DateTime, &item.OriginChannel, &item.FollowUpStatus, &notes, &item.UpdatedAt)
	item.ClientPhone = stringPtr(phone)
	item.PropertyID = stringPtr(propertyID)
	item.PropertyTitle = stringPtr(propertyTitle)
	item.AssignedAgent = stringPtr(agentID)
	item.AgentName = stringPtr(agentName)
	item.InternalNotes = stringPtr(notes)
	return item, err
}

func (s *Store) AssignInquiry(ctx context.Context, id string, agentID *string, status, notes *string) (InquiryDetail, error) {
	if s.DB == nil {
		return InquiryDetail{}, errors.New("database is not configured")
	}
	var agent interface{}
	if agentID != nil {
		agent = *agentID
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE consultas SET agente_asignado_id = CASE WHEN $2::text = '' THEN NULL ELSE COALESCE($2::text::uuid, agente_asignado_id) END, estado_seguimiento = COALESCE($3, estado_seguimiento), notas_internas = COALESCE($4, notas_internas), updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id, agent, status, notes)
	if err != nil {
		return InquiryDetail{}, err
	}
	return s.GetInquiryDetail(ctx, id)
}

type Dashboard struct {
	ActiveProperties int             `json:"inmueblesActivos"`
	HiddenProperties int             `json:"inmueblesOcultos"`
	PendingInquiries int             `json:"consultasPendientes"`
	NewInquiries     int             `json:"consultasUltimos7Dias"`
	ActiveUsers      int             `json:"usuariosActivos"`
	RecentInquiries  []InquiryDetail `json:"ultimasConsultas"`
}

func (s *Store) Dashboard(ctx context.Context, agentID string) (Dashboard, error) {
	if s.DB == nil {
		return Dashboard{}, errors.New("database is not configured")
	}
	var d Dashboard
	scope := ""
	args := []interface{}{}
	if agentID != "" {
		scope = " AND agente_id = $1"
		args = append(args, agentID)
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE estado = 'Disponible' AND deleted_at IS NULL), COUNT(*) FILTER (WHERE estado = 'Oculto' AND deleted_at IS NULL) FROM inmuebles WHERE 1 = 1`+scope, args...).Scan(&d.ActiveProperties, &d.HiddenProperties); err != nil {
		return Dashboard{}, err
	}
	scope = ""
	args = []interface{}{}
	if agentID != "" {
		scope = " AND agente_asignado_id = $1"
		args = append(args, agentID)
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FILTER (WHERE estado_seguimiento = 'Pendiente'), COUNT(*) FILTER (WHERE fecha_hora >= CURRENT_TIMESTAMP - INTERVAL '7 days') FROM consultas WHERE 1 = 1`+scope, args...).Scan(&d.PendingInquiries, &d.NewInquiries); err != nil {
		return Dashboard{}, err
	}
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usuarios u JOIN personas p ON p.id = u.id WHERE u.estado_operativo = 'Activo' AND p.deleted_at IS NULL`).Scan(&d.ActiveUsers); err != nil {
		return Dashboard{}, err
	}
	d.RecentInquiries, _, _ = s.ListInquiryDetails(ctx, InquiryFilter{Page: 1, PageSize: 5}, agentID)
	if d.RecentInquiries == nil {
		d.RecentInquiries = []InquiryDetail{}
	}
	return d, nil
}

func (s *Store) ListAgents(ctx context.Context) ([]User, error) {
	if s.DB == nil {
		return nil, errors.New("database is not configured")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT p.id, p.nombre, p.apellido, p.dni, p.correo_electronico, u.rol, u.estado_operativo, u.fecha_alta, u.ultimo_acceso FROM usuarios u JOIN personas p ON p.id = u.id WHERE p.deleted_at IS NULL AND u.estado_operativo = 'Activo' AND u.rol IN ('Agente', 'Administrador') ORDER BY p.nombre, p.apellido`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.DNI, &user.Email, &user.Role, &user.Operational, &user.CreatedAt, &user.LastAccess); err != nil {
			return nil, err
		}
		result = append(result, user)
	}
	return result, rows.Err()
}
