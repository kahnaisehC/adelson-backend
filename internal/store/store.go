package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var ErrNotFound = errors.New("resource not found")

type Store struct {
	DB *sql.DB
}

func Open(databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return &Store{}, nil
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	return &Store{DB: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.DB == nil {
		return errors.New("database is not configured")
	}
	return s.DB.PingContext(ctx)
}

type PropertyFilter struct {
	Operation      string
	PropertyType   string
	Status         string
	Currency       string
	MinPrice       *float64
	MaxPrice       *float64
	MinBedrooms    *int
	MinBathrooms   *int
	MinTotalArea   *float64
	MaxTotalArea   *float64
	City           string
	Neighborhood   string
	Query          string
	AgentID        string
	IncludeDeleted bool
	Page           int
	PageSize       int
}

type Image struct {
	ID           string    `json:"id"`
	PropertyID   string    `json:"inmuebleId"`
	FileURL      string    `json:"urlArchivo"`
	DisplayOrder int       `json:"ordenVisualizacion"`
	IsCover      bool      `json:"esPortada"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Property struct {
	ID              string                 `json:"id"`
	AgentID         string                 `json:"agenteId"`
	Title           string                 `json:"titulo"`
	Description     *string                `json:"descripcion,omitempty"`
	Operation       string                 `json:"tipoOperacion"`
	PropertyType    string                 `json:"tipoInmueble"`
	Status          string                 `json:"estado"`
	Price           float64                `json:"precio"`
	Currency        string                 `json:"moneda"`
	Expenses        *float64               `json:"montoExpensas,omitempty"`
	Rooms           *int                   `json:"cantAmbientes,omitempty"`
	Bedrooms        *int                   `json:"cantHabitaciones,omitempty"`
	Bathrooms       *int                   `json:"cantBanos,omitempty"`
	Garages         *int                   `json:"cantCocheras,omitempty"`
	TotalArea       *float64               `json:"superficieTotal,omitempty"`
	CoveredArea     *float64               `json:"superficieCubierta,omitempty"`
	Street          *string                `json:"calle,omitempty"`
	Number          *string                `json:"numero,omitempty"`
	Neighborhood    *string                `json:"barrio,omitempty"`
	City            *string                `json:"ciudad,omitempty"`
	Latitude        *float64               `json:"latitud,omitempty"`
	Longitude       *float64               `json:"longitud,omitempty"`
	Characteristics map[string]interface{} `json:"caracteristicas"`
	Images          []Image                `json:"imagenes,omitempty"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
	DeletedAt       *time.Time             `json:"deletedAt,omitempty"`
}

type PropertyInput struct {
	AgentID         string
	Title           string
	Description     *string
	Operation       string
	PropertyType    string
	Status          string
	Price           float64
	Currency        string
	Expenses        *float64
	Rooms           *int
	Bedrooms        *int
	Bathrooms       *int
	Garages         *int
	TotalArea       *float64
	CoveredArea     *float64
	Street          *string
	Number          *string
	Neighborhood    *string
	City            *string
	Latitude        *float64
	Longitude       *float64
	Characteristics map[string]interface{}
}

func (s *Store) ListProperties(ctx context.Context, f PropertyFilter) ([]Property, int, error) {
	if s.DB == nil {
		return nil, 0, errors.New("database is not configured")
	}
	where := []string{}
	args := []interface{}{}
	add := func(condition string, value interface{}) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(condition, len(args)))
	}

	if !f.IncludeDeleted {
		where = append(where, "i.deleted_at IS NULL")
	}
	if f.Operation != "" {
		add("i.tipo_operacion = $%d", f.Operation)
	}
	if f.PropertyType != "" {
		add("i.tipo_inmueble = $%d", f.PropertyType)
	}
	if f.Status != "" {
		add("i.estado = $%d", f.Status)
	} else if !f.IncludeDeleted {
		where = append(where, "i.estado = 'Disponible'")
	}
	if f.Currency != "" {
		add("i.moneda = $%d", f.Currency)
	}
	if f.MinPrice != nil {
		add("i.precio >= $%d", *f.MinPrice)
	}
	if f.MaxPrice != nil {
		add("i.precio <= $%d", *f.MaxPrice)
	}
	if f.MinBedrooms != nil {
		add("COALESCE(i.cant_habitaciones, 0) >= $%d", *f.MinBedrooms)
	}
	if f.MinBathrooms != nil {
		add("COALESCE(i.cant_banos, 0) >= $%d", *f.MinBathrooms)
	}
	if f.MinTotalArea != nil {
		add("i.superficie_total >= $%d", *f.MinTotalArea)
	}
	if f.MaxTotalArea != nil {
		add("i.superficie_total <= $%d", *f.MaxTotalArea)
	}
	if f.City != "" {
		add("i.ciudad ILIKE $%d", "%"+f.City+"%")
	}
	if f.Neighborhood != "" {
		add("i.barrio ILIKE $%d", "%"+f.Neighborhood+"%")
	}
	if f.Query != "" {
		term := "%" + f.Query + "%"
		args = append(args, term)
		first := len(args)
		args = append(args, term, term, term)
		where = append(where, fmt.Sprintf("(i.titulo ILIKE $%d OR COALESCE(i.descripcion, '') ILIKE $%d OR COALESCE(i.ciudad, '') ILIKE $%d OR COALESCE(i.barrio, '') ILIKE $%d)", first, first+1, first+2, first+3))
	}
	if f.AgentID != "" {
		add("i.agente_id = $%d", f.AgentID)
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	countQuery := "SELECT COUNT(*) FROM inmuebles i " + whereSQL
	var total int
	if err := s.DB.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count properties: %w", err)
	}

	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 20
	}
	offset := (f.Page - 1) * f.PageSize
	query := `SELECT i.id, i.agente_id, i.titulo, i.descripcion, i.tipo_operacion,
        i.tipo_inmueble, i.estado, i.precio, i.moneda, i.monto_expensas,
        i.cant_ambientes, i.cant_habitaciones, i.cant_banos, i.cant_cocheras,
        i.superficie_total, i.superficie_cubierta, i.calle, i.numero, i.barrio,
        i.ciudad, i.latitud, i.longitud, i.caracteristicas, i.created_at,
        i.updated_at, i.deleted_at
        FROM inmuebles i ` + whereSQL + ` ORDER BY i.created_at DESC LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)
	args = append(args, f.PageSize, offset)
	rows, err := s.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list properties: %w", err)
	}
	defer rows.Close()

	properties := []Property{}
	for rows.Next() {
		property, err := scanProperty(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan property: %w", err)
		}
		properties = append(properties, property)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate properties: %w", err)
	}
	return properties, total, nil
}

func (s *Store) GetProperty(ctx context.Context, id string, includeDeleted bool) (Property, error) {
	if s.DB == nil {
		return Property{}, errors.New("database is not configured")
	}
	query := `SELECT i.id, i.agente_id, i.titulo, i.descripcion, i.tipo_operacion,
        i.tipo_inmueble, i.estado, i.precio, i.moneda, i.monto_expensas,
        i.cant_ambientes, i.cant_habitaciones, i.cant_banos, i.cant_cocheras,
        i.superficie_total, i.superficie_cubierta, i.calle, i.numero, i.barrio,
        i.ciudad, i.latitud, i.longitud, i.caracteristicas, i.created_at,
        i.updated_at, i.deleted_at FROM inmuebles i WHERE i.id = $1`
	if !includeDeleted {
		query += " AND i.deleted_at IS NULL"
	}
	property, err := scanProperty(s.DB.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Property{}, ErrNotFound
	}
	if err != nil {
		return Property{}, fmt.Errorf("get property: %w", err)
	}
	property.Images, err = s.ListImages(ctx, id)
	if err != nil {
		return Property{}, err
	}
	return property, nil
}

func scanProperty(scanner interface{ Scan(...interface{}) error }) (Property, error) {
	var p Property
	var description, street, number, neighborhood, city sql.NullString
	var expenses sql.NullFloat64
	var deletedAt sql.NullTime
	var rooms, bedrooms, bathrooms, garages sql.NullInt64
	var totalArea, coveredArea, latitude, longitude sql.NullFloat64
	var characteristics []byte
	if err := scanner.Scan(&p.ID, &p.AgentID, &p.Title, &description, &p.Operation,
		&p.PropertyType, &p.Status, &p.Price, &p.Currency, &expenses, &rooms,
		&bedrooms, &bathrooms, &garages, &totalArea, &coveredArea, &street, &number,
		&neighborhood, &city, &latitude, &longitude, &characteristics, &p.CreatedAt,
		&p.UpdatedAt, &deletedAt); err != nil {
		return Property{}, err
	}
	p.Description = stringPtr(description)
	p.Expenses = floatPtr(expenses)
	p.Rooms = intPtr(rooms)
	p.Bedrooms = intPtr(bedrooms)
	p.Bathrooms = intPtr(bathrooms)
	p.Garages = intPtr(garages)
	p.TotalArea = floatPtr(totalArea)
	p.CoveredArea = floatPtr(coveredArea)
	p.Street = stringPtr(street)
	p.Number = stringPtr(number)
	p.Neighborhood = stringPtr(neighborhood)
	p.City = stringPtr(city)
	p.Latitude = floatPtr(latitude)
	p.Longitude = floatPtr(longitude)
	if deletedAt.Valid {
		p.DeletedAt = &deletedAt.Time
	}
	p.Characteristics = map[string]interface{}{}
	if len(characteristics) > 0 {
		_ = json.Unmarshal(characteristics, &p.Characteristics)
	}
	return p, nil
}

func (s *Store) CreateProperty(ctx context.Context, input PropertyInput) (Property, error) {
	if s.DB == nil {
		return Property{}, errors.New("database is not configured")
	}
	characteristics, err := json.Marshal(input.Characteristics)
	if err != nil {
		return Property{}, fmt.Errorf("marshal characteristics: %w", err)
	}
	query := `INSERT INTO inmuebles
        (agente_id, titulo, descripcion, tipo_operacion, tipo_inmueble, estado,
         precio, moneda, monto_expensas, cant_ambientes, cant_habitaciones,
         cant_banos, cant_cocheras, superficie_total, superficie_cubierta, calle,
         numero, barrio, ciudad, latitud, longitud, caracteristicas)
		VALUES ($1, $2, $3, $4, $5, $6,
         $7, $8, COALESCE($9, 0), $10, $11, $12, $13, $14, $15, $16, $17,
         $18, $19, $20, $21, $22) RETURNING id`
	var id string
	err = s.DB.QueryRowContext(ctx, query, input.AgentID, input.Title, input.Description,
		input.Operation, input.PropertyType, input.Status, input.Price, input.Currency,
		input.Expenses, input.Rooms, input.Bedrooms, input.Bathrooms, input.Garages,
		input.TotalArea, input.CoveredArea, input.Street, input.Number, input.Neighborhood,
		input.City, input.Latitude, input.Longitude, characteristics).Scan(&id)
	if err != nil {
		return Property{}, fmt.Errorf("create property: %w", err)
	}
	return s.GetProperty(ctx, id, false)
}

func (s *Store) UpdateProperty(ctx context.Context, id string, fields map[string]interface{}) (Property, error) {
	if s.DB == nil {
		return Property{}, errors.New("database is not configured")
	}
	allowed := map[string]string{
		"agenteId": "agente_id", "titulo": "titulo", "descripcion": "descripcion",
		"tipoOperacion": "tipo_operacion", "tipoInmueble": "tipo_inmueble", "estado": "estado",
		"precio": "precio", "moneda": "moneda", "montoExpensas": "monto_expensas",
		"cantAmbientes": "cant_ambientes", "cantHabitaciones": "cant_habitaciones",
		"cantBanos": "cant_banos", "cantCocheras": "cant_cocheras", "superficieTotal": "superficie_total",
		"superficieCubierta": "superficie_cubierta", "calle": "calle", "numero": "numero",
		"barrio": "barrio", "ciudad": "ciudad", "latitud": "latitud", "longitud": "longitud",
		"caracteristicas": "caracteristicas",
	}
	sets := []string{}
	args := []interface{}{}
	for field, value := range fields {
		column, ok := allowed[field]
		if !ok {
			continue
		}
		if field == "caracteristicas" {
			encoded, err := json.Marshal(value)
			if err != nil {
				return Property{}, fmt.Errorf("marshal characteristics: %w", err)
			}
			value = encoded
		}
		args = append(args, value)
		sets = append(sets, fmt.Sprintf("%s = $%d", column, len(args)))
	}
	if len(sets) == 0 {
		return s.GetProperty(ctx, id, false)
	}
	args = append(args, id)
	query := "UPDATE inmuebles SET " + strings.Join(sets, ", ") + ", updated_at = CURRENT_TIMESTAMP WHERE id = $" + fmt.Sprint(len(args)) + " AND deleted_at IS NULL"
	result, err := s.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return Property{}, fmt.Errorf("update property: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Property{}, ErrNotFound
	}
	return s.GetProperty(ctx, id, false)
}

func (s *Store) DeleteProperty(ctx context.Context, id string) error {
	if s.DB == nil {
		return errors.New("database is not configured")
	}
	result, err := s.DB.ExecContext(ctx, `UPDATE inmuebles SET deleted_at = CURRENT_TIMESTAMP, estado = 'Oculto', updated_at = CURRENT_TIMESTAMP WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("delete property: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListImages(ctx context.Context, propertyID string) ([]Image, error) {
	if s.DB == nil {
		return nil, errors.New("database is not configured")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, inmueble_id, url_archivo, orden_visualizacion, es_portada, created_at FROM imagenes WHERE inmueble_id = $1 ORDER BY orden_visualizacion, created_at`, propertyID)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()
	images := []Image{}
	for rows.Next() {
		var image Image
		if err := rows.Scan(&image.ID, &image.PropertyID, &image.FileURL, &image.DisplayOrder, &image.IsCover, &image.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan image: %w", err)
		}
		images = append(images, image)
	}
	return images, rows.Err()
}

func (s *Store) AddImage(ctx context.Context, propertyID string, image Image) (Image, error) {
	if s.DB == nil {
		return Image{}, errors.New("database is not configured")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Image{}, err
	}
	defer tx.Rollback()
	if image.IsCover {
		if _, err := tx.ExecContext(ctx, `UPDATE imagenes SET es_portada = false WHERE inmueble_id = $1`, propertyID); err != nil {
			return Image{}, fmt.Errorf("clear cover: %w", err)
		}
	}
	var result Image
	err = tx.QueryRowContext(ctx, `INSERT INTO imagenes (inmueble_id, url_archivo, orden_visualizacion, es_portada) VALUES ($1, $2, COALESCE(NULLIF($3, 0), 1), $4) RETURNING id, inmueble_id, url_archivo, orden_visualizacion, es_portada, created_at`, propertyID, image.FileURL, image.DisplayOrder, image.IsCover).Scan(&result.ID, &result.PropertyID, &result.FileURL, &result.DisplayOrder, &result.IsCover, &result.CreatedAt)
	if err != nil {
		return Image{}, fmt.Errorf("add image: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Image{}, err
	}
	return result, nil
}

type Client struct {
	ID                   string    `json:"id"`
	FirstName            string    `json:"nombre"`
	LastName             string    `json:"apellido"`
	DNI                  string    `json:"dni"`
	Email                string    `json:"correoElectronico"`
	ConsentNotifications bool      `json:"consentimientoNotificaciones"`
	LeadStatus           string    `json:"estadoLead"`
	PrimaryClientType    *string   `json:"tipoClientePrincipal,omitempty"`
	RegistrationDate     time.Time `json:"fechaRegistro"`
	LastInteraction      time.Time `json:"fechaUltimaInteraccion"`
}

type User struct {
	ID          string     `json:"id"`
	FirstName   string     `json:"nombre"`
	LastName    string     `json:"apellido"`
	DNI         string     `json:"dni"`
	Email       string     `json:"correoElectronico"`
	Role        string     `json:"rol"`
	Operational string     `json:"estadoOperativo"`
	CreatedAt   time.Time  `json:"fechaAlta"`
	LastAccess  *time.Time `json:"ultimoAcceso,omitempty"`
}

type UserInput struct {
	FirstName string
	LastName  string
	DNI       string
	Email     string
	Hash      string
	Role      string
}

func (s *Store) ListUsers(ctx context.Context, page, pageSize int) ([]User, int, error) {
	if s.DB == nil {
		return nil, 0, errors.New("database is not configured")
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM usuarios u JOIN personas p ON p.id = u.id WHERE p.deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT p.id, p.nombre, p.apellido, p.dni, p.correo_electronico, u.rol, u.estado_operativo, u.fecha_alta, u.ultimo_acceso FROM usuarios u JOIN personas p ON p.id = u.id WHERE p.deleted_at IS NULL ORDER BY u.fecha_alta DESC LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	users := []User{}
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.FirstName, &user.LastName, &user.DNI, &user.Email, &user.Role, &user.Operational, &user.CreatedAt, &user.LastAccess); err != nil {
			return nil, 0, err
		}
		users = append(users, user)
	}
	return users, total, rows.Err()
}

func (s *Store) GetUser(ctx context.Context, id string) (User, error) {
	if s.DB == nil {
		return User{}, errors.New("database is not configured")
	}
	var user User
	err := s.DB.QueryRowContext(ctx, `SELECT p.id, p.nombre, p.apellido, p.dni, p.correo_electronico, u.rol, u.estado_operativo, u.fecha_alta, u.ultimo_acceso FROM usuarios u JOIN personas p ON p.id = u.id WHERE u.id = $1 AND p.deleted_at IS NULL`, id).Scan(&user.ID, &user.FirstName, &user.LastName, &user.DNI, &user.Email, &user.Role, &user.Operational, &user.CreatedAt, &user.LastAccess)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return user, err
}

func (s *Store) CreateUser(ctx context.Context, input UserInput) (User, error) {
	if s.DB == nil {
		return User{}, errors.New("database is not configured")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	var id string
	if err := tx.QueryRowContext(ctx, `INSERT INTO personas (nombre, apellido, dni, correo_electronico, contrasena_hash) VALUES ($1, $2, $3, $4, $5) RETURNING id`, input.FirstName, input.LastName, input.DNI, input.Email, input.Hash).Scan(&id); err != nil {
		return User{}, fmt.Errorf("create user person: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO usuarios (id, rol) VALUES ($1, $2)`, id, input.Role); err != nil {
		return User{}, fmt.Errorf("create user profile: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return s.GetUser(ctx, id)
}

func (s *Store) UpdateUser(ctx context.Context, id string, fields map[string]interface{}) (User, error) {
	if s.DB == nil {
		return User{}, errors.New("database is not configured")
	}
	personColumns := map[string]string{"nombre": "nombre", "apellido": "apellido", "correoElectronico": "correo_electronico", "dni": "dni"}
	userColumns := map[string]string{"rol": "rol", "estadoOperativo": "estado_operativo"}
	personSets, userSets, args := []string{}, []string{}, []interface{}{}
	for field, value := range fields {
		if column, ok := personColumns[field]; ok {
			args = append(args, value)
			personSets = append(personSets, fmt.Sprintf("%s = $%d", column, len(args)))
		}
		if column, ok := userColumns[field]; ok {
			args = append(args, value)
			userSets = append(userSets, fmt.Sprintf("%s = $%d", column, len(args)))
		}
	}
	if len(personSets) == 0 && len(userSets) == 0 {
		return s.GetUser(ctx, id)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()
	if len(personSets) > 0 {
		personSets = append(personSets, "updated_at = CURRENT_TIMESTAMP")
		args = append(args, id)
		if _, err := tx.ExecContext(ctx, "UPDATE personas SET "+strings.Join(personSets, ", ")+" WHERE id = $"+fmt.Sprint(len(args))+" AND deleted_at IS NULL", args...); err != nil {
			return User{}, err
		}
		args = args[:len(args)-1]
	}
	if len(userSets) > 0 {
		args = append(args, id)
		if _, err := tx.ExecContext(ctx, "UPDATE usuarios SET "+strings.Join(userSets, ", ")+" WHERE id = $"+fmt.Sprint(len(args)), args...); err != nil {
			return User{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return s.GetUser(ctx, id)
}

type ClientInput struct {
	FirstName            string
	LastName             string
	DNI                  string
	Email                string
	PasswordHash         string
	ConsentNotifications bool
	PrimaryClientType    *string
}

func (s *Store) GetClientByEmail(ctx context.Context, email string) (Client, error) {
	if s.DB == nil {
		return Client{}, errors.New("database is not configured")
	}
	row := s.DB.QueryRowContext(ctx, `SELECT p.id, p.nombre, p.apellido, p.dni, p.correo_electronico, c.consentimiento_notificaciones, c.estado_lead, c.tipo_cliente_principal, c.fecha_registro, c.fecha_ultima_interaccion FROM clientes c JOIN personas p ON p.id = c.id WHERE lower(p.correo_electronico) = lower($1) AND p.deleted_at IS NULL`, email)
	client, err := scanClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	return client, err
}

type Phone struct {
	ID       string `json:"id"`
	ClientID string `json:"personaId"`
	Number   string `json:"numero"`
	Primary  bool   `json:"esPrincipal"`
}

func (s *Store) ListPhones(ctx context.Context, clientID string) ([]Phone, error) {
	if s.DB == nil {
		return nil, errors.New("database is not configured")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, persona_id, numero, es_principal FROM telefonos_persona WHERE persona_id = $1 ORDER BY es_principal DESC, id`, clientID)
	if err != nil {
		return nil, fmt.Errorf("list phones: %w", err)
	}
	defer rows.Close()
	phones := []Phone{}
	for rows.Next() {
		var phone Phone
		if err := rows.Scan(&phone.ID, &phone.ClientID, &phone.Number, &phone.Primary); err != nil {
			return nil, err
		}
		phones = append(phones, phone)
	}
	return phones, rows.Err()
}

func (s *Store) CreatePhone(ctx context.Context, clientID, number string, primary bool) (Phone, error) {
	if s.DB == nil {
		return Phone{}, errors.New("database is not configured")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Phone{}, err
	}
	defer tx.Rollback()
	if primary {
		if _, err := tx.ExecContext(ctx, `UPDATE telefonos_persona SET es_principal = false WHERE persona_id = $1`, clientID); err != nil {
			return Phone{}, err
		}
	}
	var phone Phone
	err = tx.QueryRowContext(ctx, `INSERT INTO telefonos_persona (persona_id, numero, es_principal) VALUES ($1, $2, $3) RETURNING id, persona_id, numero, es_principal`, clientID, number, primary).Scan(&phone.ID, &phone.ClientID, &phone.Number, &phone.Primary)
	if err != nil {
		return Phone{}, fmt.Errorf("create phone: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Phone{}, err
	}
	return phone, nil
}

func (s *Store) DeletePhone(ctx context.Context, clientID, phoneID string) error {
	if s.DB == nil {
		return errors.New("database is not configured")
	}
	result, err := s.DB.ExecContext(ctx, `DELETE FROM telefonos_persona WHERE id = $1 AND persona_id = $2`, phoneID, clientID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListClients(ctx context.Context, page, pageSize int) ([]Client, int, error) {
	if s.DB == nil {
		return nil, 0, errors.New("database is not configured")
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM clientes c JOIN personas p ON p.id = c.id WHERE p.deleted_at IS NULL`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count clients: %w", err)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT p.id, p.nombre, p.apellido, p.dni, p.correo_electronico, c.consentimiento_notificaciones, c.estado_lead, c.tipo_cliente_principal, c.fecha_registro, c.fecha_ultima_interaccion FROM clientes c JOIN personas p ON p.id = c.id WHERE p.deleted_at IS NULL ORDER BY c.fecha_registro DESC LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list clients: %w", err)
	}
	defer rows.Close()
	clients := []Client{}
	for rows.Next() {
		client, err := scanClient(rows)
		if err != nil {
			return nil, 0, err
		}
		clients = append(clients, client)
	}
	return clients, total, rows.Err()
}

func scanClient(scanner interface{ Scan(...interface{}) error }) (Client, error) {
	var c Client
	var clientType sql.NullString
	if err := scanner.Scan(&c.ID, &c.FirstName, &c.LastName, &c.DNI, &c.Email, &c.ConsentNotifications, &c.LeadStatus, &clientType, &c.RegistrationDate, &c.LastInteraction); err != nil {
		return Client{}, err
	}
	if clientType.Valid {
		c.PrimaryClientType = &clientType.String
	}
	return c, nil
}

func (s *Store) GetClient(ctx context.Context, id string) (Client, error) {
	if s.DB == nil {
		return Client{}, errors.New("database is not configured")
	}
	row := s.DB.QueryRowContext(ctx, `SELECT p.id, p.nombre, p.apellido, p.dni, p.correo_electronico, c.consentimiento_notificaciones, c.estado_lead, c.tipo_cliente_principal, c.fecha_registro, c.fecha_ultima_interaccion FROM clientes c JOIN personas p ON p.id = c.id WHERE c.id = $1 AND p.deleted_at IS NULL`, id)
	c, err := scanClient(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Client{}, ErrNotFound
	}
	if err != nil {
		return Client{}, fmt.Errorf("get client: %w", err)
	}
	return c, nil
}

func (s *Store) CreateClient(ctx context.Context, input ClientInput) (Client, error) {
	if s.DB == nil {
		return Client{}, errors.New("database is not configured")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Client{}, err
	}
	defer tx.Rollback()
	var id string
	if err := tx.QueryRowContext(ctx, `INSERT INTO personas (nombre, apellido, dni, correo_electronico, contrasena_hash) VALUES ($1, $2, $3, $4, $5) RETURNING id`, input.FirstName, input.LastName, input.DNI, input.Email, input.PasswordHash).Scan(&id); err != nil {
		return Client{}, fmt.Errorf("create person: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO clientes (id, consentimiento_notificaciones, tipo_cliente_principal) VALUES ($1, $2, $3)`, id, input.ConsentNotifications, input.PrimaryClientType); err != nil {
		return Client{}, fmt.Errorf("create client: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Client{}, err
	}
	return s.GetClient(ctx, id)
}

func (s *Store) UpdateClient(ctx context.Context, id string, fields map[string]interface{}) (Client, error) {
	if s.DB == nil {
		return Client{}, errors.New("database is not configured")
	}
	personFields := map[string]string{"nombre": "nombre", "apellido": "apellido", "correoElectronico": "correo_electronico", "dni": "dni"}
	clientFields := map[string]string{"consentimientoNotificaciones": "consentimiento_notificaciones", "estadoLead": "estado_lead", "tipoClientePrincipal": "tipo_cliente_principal"}
	personSets, clientSets, args := []string{}, []string{}, []interface{}{}
	for field, value := range fields {
		if column, ok := personFields[field]; ok {
			args = append(args, value)
			personSets = append(personSets, fmt.Sprintf("%s = $%d", column, len(args)))
		}
		if column, ok := clientFields[field]; ok {
			args = append(args, value)
			clientSets = append(clientSets, fmt.Sprintf("%s = $%d", column, len(args)))
		}
	}
	if len(personSets) == 0 && len(clientSets) == 0 {
		return s.GetClient(ctx, id)
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return Client{}, err
	}
	defer tx.Rollback()
	if len(personSets) > 0 {
		personSets = append(personSets, "updated_at = CURRENT_TIMESTAMP")
		args = append(args, id)
		if _, err := tx.ExecContext(ctx, "UPDATE personas SET "+strings.Join(personSets, ", ")+" WHERE id = $"+fmt.Sprint(len(args))+" AND deleted_at IS NULL", args...); err != nil {
			return Client{}, fmt.Errorf("update person: %w", err)
		}
		args = args[:len(args)-1]
	}
	if len(clientSets) > 0 {
		args = append(args, id)
		if _, err := tx.ExecContext(ctx, "UPDATE clientes SET "+strings.Join(clientSets, ", ")+" WHERE id = $"+fmt.Sprint(len(args)), args...); err != nil {
			return Client{}, fmt.Errorf("update client: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return Client{}, err
	}
	return s.GetClient(ctx, id)
}

type Preference struct {
	ID            string    `json:"id"`
	ClientID      string    `json:"clienteId"`
	MinPrice      *float64  `json:"rangoPrecioMin,omitempty"`
	MaxPrice      *float64  `json:"rangoPrecioMax,omitempty"`
	Currency      *string   `json:"moneda,omitempty"`
	Operation     *string   `json:"tipoOperacion,omitempty"`
	PropertyType  *string   `json:"tipoInmueble,omitempty"`
	MinBedrooms   *int      `json:"cantidadDormitoriosMin,omitempty"`
	PreferredArea *string   `json:"zonaPreferida,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

type PreferenceInput struct {
	MinPrice      *float64
	MaxPrice      *float64
	Currency      *string
	Operation     *string
	PropertyType  *string
	MinBedrooms   *int
	PreferredArea *string
}

func (s *Store) ListPreferences(ctx context.Context, clientID string) ([]Preference, error) {
	if s.DB == nil {
		return nil, errors.New("database is not configured")
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, cliente_id, rango_precio_min, rango_precio_max, moneda, tipo_operacion, tipo_inmueble, cantidad_dormitorios_min, zona_preferida, created_at FROM preferencias_busquedas WHERE cliente_id = $1 ORDER BY created_at DESC`, clientID)
	if err != nil {
		return nil, fmt.Errorf("list preferences: %w", err)
	}
	defer rows.Close()
	result := []Preference{}
	for rows.Next() {
		var p Preference
		var min, max sql.NullFloat64
		var currency, operation, propertyType, zone sql.NullString
		var bedrooms sql.NullInt64
		if err := rows.Scan(&p.ID, &p.ClientID, &min, &max, &currency, &operation, &propertyType, &bedrooms, &zone, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.MinPrice, p.MaxPrice, p.Currency, p.Operation, p.PropertyType, p.MinBedrooms, p.PreferredArea = floatPtr(min), floatPtr(max), stringPtr(currency), stringPtr(operation), stringPtr(propertyType), intPtr(bedrooms), stringPtr(zone)
		result = append(result, p)
	}
	return result, rows.Err()
}

func (s *Store) CreatePreference(ctx context.Context, clientID string, input PreferenceInput) (Preference, error) {
	if s.DB == nil {
		return Preference{}, errors.New("database is not configured")
	}
	var p Preference
	var min, max sql.NullFloat64
	var currency, operation, propertyType, zone sql.NullString
	var bedrooms sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `INSERT INTO preferencias_busquedas (cliente_id, rango_precio_min, rango_precio_max, moneda, tipo_operacion, tipo_inmueble, cantidad_dormitorios_min, zona_preferida) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, cliente_id, rango_precio_min, rango_precio_max, moneda, tipo_operacion, tipo_inmueble, cantidad_dormitorios_min, zona_preferida, created_at`, clientID, input.MinPrice, input.MaxPrice, input.Currency, input.Operation, input.PropertyType, input.MinBedrooms, input.PreferredArea).Scan(&p.ID, &p.ClientID, &min, &max, &currency, &operation, &propertyType, &bedrooms, &zone, &p.CreatedAt)
	if err != nil {
		return Preference{}, fmt.Errorf("create preference: %w", err)
	}
	p.MinPrice, p.MaxPrice, p.Currency, p.Operation, p.PropertyType, p.MinBedrooms, p.PreferredArea = floatPtr(min), floatPtr(max), stringPtr(currency), stringPtr(operation), stringPtr(propertyType), intPtr(bedrooms), stringPtr(zone)
	return p, nil
}

type Inquiry struct {
	ID             string    `json:"id"`
	ClientID       string    `json:"clienteId"`
	PropertyID     string    `json:"inmuebleId"`
	DateTime       time.Time `json:"fechaHora"`
	OriginChannel  string    `json:"canalOrigen"`
	FollowUpStatus string    `json:"estadoSeguimiento"`
	InternalNotes  *string   `json:"notasInternas,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func (s *Store) CreateInquiry(ctx context.Context, clientID, propertyID, channel string, notes *string) (Inquiry, error) {
	if s.DB == nil {
		return Inquiry{}, errors.New("database is not configured")
	}
	var inquiry Inquiry
	err := s.DB.QueryRowContext(ctx, `INSERT INTO consultas (cliente_id, inmueble_id, canal_origen, notas_internas) VALUES ($1, $2, $3, $4) RETURNING id, cliente_id, inmueble_id, fecha_hora, canal_origen, estado_seguimiento, notas_internas, updated_at`, clientID, propertyID, channel, notes).Scan(&inquiry.ID, &inquiry.ClientID, &inquiry.PropertyID, &inquiry.DateTime, &inquiry.OriginChannel, &inquiry.FollowUpStatus, &inquiry.InternalNotes, &inquiry.UpdatedAt)
	if err != nil {
		return Inquiry{}, fmt.Errorf("create inquiry: %w", err)
	}
	return inquiry, nil
}

func (s *Store) ListInquiries(ctx context.Context, page, pageSize int) ([]Inquiry, int, error) {
	if s.DB == nil {
		return nil, 0, errors.New("database is not configured")
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM consultas`).Scan(&total); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT id, cliente_id, inmueble_id, fecha_hora, canal_origen, estado_seguimiento, notas_internas, updated_at FROM consultas ORDER BY fecha_hora DESC LIMIT $1 OFFSET $2`, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []Inquiry{}
	for rows.Next() {
		var i Inquiry
		if err := rows.Scan(&i.ID, &i.ClientID, &i.PropertyID, &i.DateTime, &i.OriginChannel, &i.FollowUpStatus, &i.InternalNotes, &i.UpdatedAt); err != nil {
			return nil, 0, err
		}
		result = append(result, i)
	}
	return result, total, rows.Err()
}

func (s *Store) UpdateInquiry(ctx context.Context, id string, status, notes *string) (Inquiry, error) {
	if s.DB == nil {
		return Inquiry{}, errors.New("database is not configured")
	}
	var inquiry Inquiry
	err := s.DB.QueryRowContext(ctx, `UPDATE consultas SET estado_seguimiento = COALESCE($2, estado_seguimiento), notas_internas = COALESCE($3, notas_internas), updated_at = CURRENT_TIMESTAMP WHERE id = $1 RETURNING id, cliente_id, inmueble_id, fecha_hora, canal_origen, estado_seguimiento, notas_internas, updated_at`, id, status, notes).Scan(&inquiry.ID, &inquiry.ClientID, &inquiry.PropertyID, &inquiry.DateTime, &inquiry.OriginChannel, &inquiry.FollowUpStatus, &inquiry.InternalNotes, &inquiry.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Inquiry{}, ErrNotFound
	}
	if err != nil {
		return Inquiry{}, err
	}
	return inquiry, nil
}

type ChatSession struct {
	ID              string          `json:"id"`
	ClientID        string          `json:"clienteId"`
	AssignedAgentID *string         `json:"agenteAsignadoId,omitempty"`
	StartedAt       time.Time       `json:"fechaInicio"`
	LastInteraction time.Time       `json:"fechaUltimaInteraccion"`
	Status          string          `json:"estadoSesion"`
	MessageHistory  json.RawMessage `json:"historialMensajes"`
}

func (s *Store) GetChatSession(ctx context.Context, id string) (ChatSession, error) {
	if s.DB == nil {
		return ChatSession{}, errors.New("database is not configured")
	}
	var session ChatSession
	if err := s.DB.QueryRowContext(ctx, `SELECT id, cliente_id, agente_asignado_id, fecha_inicio, fecha_ultima_interaccion, estado_sesion, historial_mensajes FROM sesiones_chat WHERE id = $1`, id).Scan(&session.ID, &session.ClientID, &session.AssignedAgentID, &session.StartedAt, &session.LastInteraction, &session.Status, &session.MessageHistory); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ChatSession{}, ErrNotFound
		}
		return ChatSession{}, err
	}
	return session, nil
}

func (s *Store) CreateChatSession(ctx context.Context, clientID string) (ChatSession, error) {
	if s.DB == nil {
		return ChatSession{}, errors.New("database is not configured")
	}
	var session ChatSession
	err := s.DB.QueryRowContext(ctx, `INSERT INTO sesiones_chat (cliente_id) VALUES ($1) RETURNING id, cliente_id, agente_asignado_id, fecha_inicio, fecha_ultima_interaccion, estado_sesion, historial_mensajes`, clientID).Scan(&session.ID, &session.ClientID, &session.AssignedAgentID, &session.StartedAt, &session.LastInteraction, &session.Status, &session.MessageHistory)
	if err != nil {
		return ChatSession{}, err
	}
	return session, nil
}

func (s *Store) AppendMessage(ctx context.Context, id string, message json.RawMessage) (ChatSession, error) {
	if s.DB == nil {
		return ChatSession{}, errors.New("database is not configured")
	}
	var session ChatSession
	err := s.DB.QueryRowContext(ctx, `UPDATE sesiones_chat SET historial_mensajes = historial_mensajes || jsonb_build_array($2::jsonb), fecha_ultima_interaccion = CURRENT_TIMESTAMP WHERE id = $1 RETURNING id, cliente_id, agente_asignado_id, fecha_inicio, fecha_ultima_interaccion, estado_sesion, historial_mensajes`, id, string(message)).Scan(&session.ID, &session.ClientID, &session.AssignedAgentID, &session.StartedAt, &session.LastInteraction, &session.Status, &session.MessageHistory)
	if errors.Is(err, sql.ErrNoRows) {
		return ChatSession{}, ErrNotFound
	}
	if err != nil {
		return ChatSession{}, err
	}
	return session, nil
}

func (s *Store) TransitionChat(ctx context.Context, id, status, agentID string) (ChatSession, error) {
	if s.DB == nil {
		return ChatSession{}, errors.New("database is not configured")
	}
	var session ChatSession
	var err error
	if agentID == "" {
		err = s.DB.QueryRowContext(ctx, `UPDATE sesiones_chat SET estado_sesion = $2, agente_asignado_id = NULL, fecha_ultima_interaccion = CURRENT_TIMESTAMP WHERE id = $1 RETURNING id, cliente_id, agente_asignado_id, fecha_inicio, fecha_ultima_interaccion, estado_sesion, historial_mensajes`, id, status).Scan(&session.ID, &session.ClientID, &session.AssignedAgentID, &session.StartedAt, &session.LastInteraction, &session.Status, &session.MessageHistory)
	} else {
		err = s.DB.QueryRowContext(ctx, `UPDATE sesiones_chat SET estado_sesion = $2, agente_asignado_id = $3, fecha_ultima_interaccion = CURRENT_TIMESTAMP WHERE id = $1 RETURNING id, cliente_id, agente_asignado_id, fecha_inicio, fecha_ultima_interaccion, estado_sesion, historial_mensajes`, id, status, agentID).Scan(&session.ID, &session.ClientID, &session.AssignedAgentID, &session.StartedAt, &session.LastInteraction, &session.Status, &session.MessageHistory)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ChatSession{}, ErrNotFound
	}
	if err != nil {
		return ChatSession{}, err
	}
	return session, nil
}

func stringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
func intPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	result := int(value.Int64)
	return &result
}
func floatPtr(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	return &value.Float64
}
