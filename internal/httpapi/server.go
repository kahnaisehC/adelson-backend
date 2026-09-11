package httpapi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"adelson-backend/internal/config"
	"adelson-backend/internal/store"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	config config.Config
	store  *store.Store
	log    *slog.Logger
}

type contextKey string

const requestIDKey contextKey = "requestID"

type claims struct {
	Role      string `json:"role"`
	TokenType string `json:"tokenType"`
	jwt.RegisteredClaims
}

func New(cfg config.Config, db *store.Store, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	if db == nil {
		db = &store.Store{}
	}
	s := &Server{config: cfg, store: db, log: logger}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /readyz", s.ready)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)
	mux.HandleFunc("POST /api/v1/auth/refresh", s.refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", s.logout)
	mux.HandleFunc("GET /api/v1/auth/me", s.me)
	mux.HandleFunc("GET /api/v1/usuarios", s.users)
	mux.HandleFunc("POST /api/v1/usuarios", s.users)
	mux.HandleFunc("GET /api/v1/usuarios/{usuarioId}", s.user)
	mux.HandleFunc("PATCH /api/v1/usuarios/{usuarioId}", s.user)

	mux.HandleFunc("GET /api/v1/inmuebles", s.properties)
	mux.HandleFunc("POST /api/v1/inmuebles", s.properties)
	mux.HandleFunc("GET /api/v1/inmuebles/{inmuebleId}", s.property)
	mux.HandleFunc("PATCH /api/v1/inmuebles/{inmuebleId}", s.property)
	mux.HandleFunc("DELETE /api/v1/inmuebles/{inmuebleId}", s.property)
	mux.HandleFunc("POST /api/v1/inmuebles/{inmuebleId}/restaurar", s.notImplemented)
	mux.HandleFunc("GET /api/v1/inmuebles/{inmuebleId}/imagenes", s.images)
	mux.HandleFunc("POST /api/v1/inmuebles/{inmuebleId}/imagenes", s.images)
	mux.HandleFunc("PATCH /api/v1/inmuebles/{inmuebleId}/imagenes/{imagenId}", s.notImplemented)
	mux.HandleFunc("DELETE /api/v1/inmuebles/{inmuebleId}/imagenes/{imagenId}", s.notImplemented)
	mux.HandleFunc("POST /api/v1/inmuebles/{inmuebleId}/consultas", s.publicInquiry)

	mux.HandleFunc("GET /api/v1/clientes", s.clients)
	mux.HandleFunc("POST /api/v1/clientes", s.clients)
	mux.HandleFunc("GET /api/v1/clientes/{clienteId}", s.client)
	mux.HandleFunc("PATCH /api/v1/clientes/{clienteId}", s.client)
	mux.HandleFunc("DELETE /api/v1/clientes/{clienteId}", s.client)
	mux.HandleFunc("PATCH /api/v1/clientes/{clienteId}/consentimiento", s.consent)
	mux.HandleFunc("GET /api/v1/clientes/{clienteId}/telefonos", s.phones)
	mux.HandleFunc("POST /api/v1/clientes/{clienteId}/telefonos", s.phones)
	mux.HandleFunc("DELETE /api/v1/clientes/{clienteId}/telefonos/{telefonoId}", s.phone)
	mux.HandleFunc("GET /api/v1/clientes/{clienteId}/preferencias", s.preferences)
	mux.HandleFunc("POST /api/v1/clientes/{clienteId}/preferencias", s.preferences)
	mux.HandleFunc("GET /api/v1/clientes/{clienteId}/preferencias/{preferenciaId}", s.notImplemented)
	mux.HandleFunc("PATCH /api/v1/clientes/{clienteId}/preferencias/{preferenciaId}", s.notImplemented)
	mux.HandleFunc("DELETE /api/v1/clientes/{clienteId}/preferencias/{preferenciaId}", s.notImplemented)
	mux.HandleFunc("GET /api/v1/clientes/{clienteId}/recomendaciones", s.notImplemented)
	mux.HandleFunc("GET /api/v1/clientes/{clienteId}/contratos", s.notImplemented)
	mux.HandleFunc("POST /api/v1/clientes/{clienteId}/contratos", s.notImplemented)

	mux.HandleFunc("GET /api/v1/consultas", s.inquiries)
	mux.HandleFunc("POST /api/v1/consultas", s.inquiries)
	mux.HandleFunc("GET /api/v1/consultas/{consultaId}", s.notImplemented)
	mux.HandleFunc("PATCH /api/v1/consultas/{consultaId}", s.updateInquiry)

	mux.HandleFunc("GET /api/v1/sesiones-chat", s.sessions)
	mux.HandleFunc("POST /api/v1/sesiones-chat", s.sessions)
	mux.HandleFunc("GET /api/v1/sesiones-chat/{sesionId}", s.session)
	mux.HandleFunc("POST /api/v1/sesiones-chat/{sesionId}/mensajes", s.message)
	mux.HandleFunc("POST /api/v1/sesiones-chat/{sesionId}/handoff", s.handoff)
	mux.HandleFunc("POST /api/v1/sesiones-chat/{sesionId}/tomar-control", s.takeControl)
	mux.HandleFunc("POST /api/v1/sesiones-chat/{sesionId}/reanudar-ia", s.resumeAI)
	mux.HandleFunc("POST /api/v1/sesiones-chat/{sesionId}/cerrar", s.closeSession)

	mux.HandleFunc("GET /api/v1/webhooks/whatsapp", s.whatsappVerify)
	mux.HandleFunc("POST /api/v1/webhooks/whatsapp", s.whatsappWebhook)
	mux.HandleFunc("POST /api/v1/interno/jobs/matching", s.notImplemented)
	mux.HandleFunc("GET /api/v1/auditoria", s.notImplemented)
	mux.HandleFunc("GET /api/v1/seguimientos", s.notImplemented)

	return requestIDMiddleware(recoveryMiddleware(loggingMiddleware(mux, logger)))
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeData(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Ping(r.Context()); err != nil {
		writeError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "La base de datos no está disponible.", nil)
		return
	}
	writeData(w, r, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"correoElectronico"`
		Password string `json:"contrasena"`
	}
	if !decodeJSON(w, r, &input) || input.Email == "" || input.Password == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Correo y contraseña son obligatorios.", nil)
		return
	}
	if s.store == nil || s.store.DB == nil {
		writeError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "La base de datos no está configurada.", nil)
		return
	}
	var user struct {
		ID       string
		Name     string
		LastName string
		Email    string
		Hash     string
		Role     string
		Status   string
	}
	err := s.store.DB.QueryRowContext(r.Context(), `SELECT p.id, p.nombre, p.apellido, p.correo_electronico, p.contrasena_hash, u.rol, u.estado_operativo FROM personas p JOIN usuarios u ON u.id = p.id WHERE lower(p.correo_electronico) = lower($1) AND p.deleted_at IS NULL`, input.Email).Scan(&user.ID, &user.Name, &user.LastName, &user.Email, &user.Hash, &user.Role, &user.Status)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Hash), []byte(input.Password)) != nil || user.Status != "Activo" {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Las credenciales no son válidas.", nil)
		return
	}
	if _, err := s.store.DB.ExecContext(r.Context(), `UPDATE usuarios SET ultimo_acceso = CURRENT_TIMESTAMP WHERE id = $1`, user.ID); err != nil {
		s.log.Error("update last access", "error", err)
	}
	token, err := s.issueToken(user.ID, user.Role, "access", 15*time.Minute)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo crear la sesión.", nil)
		return
	}
	refresh, err := s.issueToken(user.ID, user.Role, "refresh", 7*24*time.Hour)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo crear la sesión.", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: refresh, Path: "/api/v1/auth", MaxAge: 7 * 24 * 60 * 60, HttpOnly: true, Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode})
	writeData(w, r, http.StatusOK, map[string]interface{}{
		"accessToken": token,
		"tokenType":   "Bearer",
		"expiresIn":   900,
		"usuario": map[string]string{
			"id": user.ID, "nombre": user.Name, "apellido": user.LastName,
			"correoElectronico": user.Email, "rol": user.Role, "estadoOperativo": user.Status,
		},
	})
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Se requiere un refresh token válido.", nil)
		return
	}
	current, ok := s.parseToken(cookie.Value, "refresh")
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Se requiere un refresh token válido.", nil)
		return
	}
	token, err := s.issueToken(current.Subject, current.Role, "access", 15*time.Minute)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo renovar la sesión.", nil)
		return
	}
	newRefresh, err := s.issueToken(current.Subject, current.Role, "refresh", 7*24*time.Hour)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo renovar la sesión.", nil)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: newRefresh, Path: "/api/v1/auth", MaxAge: 7 * 24 * 60 * 60, HttpOnly: true, Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode})
	writeData(w, r, http.StatusOK, map[string]interface{}{"accessToken": token, "tokenType": "Bearer", "expiresIn": 900})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.require(w, r); !ok {
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: "", Path: "/api/v1/auth", MaxAge: -1, HttpOnly: true, Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r)
	if !ok {
		return
	}
	writeData(w, r, http.StatusOK, map[string]string{"id": current.Subject, "rol": current.Role})
}

func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if _, ok := s.require(w, r, "Administrador"); !ok {
			return
		}
		var input userRequest
		if !decodeJSON(w, r, &input) {
			return
		}
		if err := input.validate(); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo proteger la contraseña.", nil)
			return
		}
		user, err := s.store.CreateUser(r.Context(), store.UserInput{FirstName: input.FirstName, LastName: input.LastName, DNI: input.DNI, Email: input.Email, Hash: string(hash), Role: input.Role})
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusCreated, user)
		return
	}
	if _, ok := s.require(w, r, "Administrador"); !ok {
		return
	}
	page, pageSize := pageParams(r)
	users, total, err := s.store.ListUsers(r.Context(), page, pageSize)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writePaginated(w, r, users, page, pageSize, total)
}

func (s *Server) user(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r, "Administrador", "Agente")
	if !ok {
		return
	}
	id := r.PathValue("usuarioId")
	if current.Role != "Administrador" && current.Subject != id {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "No puede acceder a este usuario.", nil)
		return
	}
	switch r.Method {
	case http.MethodGet:
		user, err := s.store.GetUser(r.Context(), id)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, user)
	case http.MethodPatch:
		var fields map[string]interface{}
		if !decodeJSON(w, r, &fields) {
			return
		}
		if current.Role != "Administrador" {
			delete(fields, "rol")
			delete(fields, "estadoOperativo")
		}
		user, err := s.store.UpdateUser(r.Context(), id, fields)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, user)
	}
}

func (s *Server) issueToken(subject, role, tokenType string, duration time.Duration) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{Role: role, TokenType: tokenType, RegisteredClaims: jwt.RegisteredClaims{
		Subject: subject, Issuer: "adelson-api", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(duration)), ID: randomID(),
	}})
	return token.SignedString([]byte(s.config.JWTSecret))
}

func (s *Server) authenticate(r *http.Request) (claims, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return claims{}, false
	}
	return s.parseToken(strings.TrimPrefix(header, "Bearer "), "access")
}

func (s *Server) parseToken(raw, expectedType string) (claims, bool) {
	token, err := jwt.ParseWithClaims(raw, &claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.config.JWTSecret), nil
	})
	if err != nil || !token.Valid {
		return claims{}, false
	}
	parsed, ok := token.Claims.(*claims)
	if !ok || parsed.TokenType != expectedType {
		return claims{}, false
	}
	return *parsed, true
}

func (s *Server) require(w http.ResponseWriter, r *http.Request, roles ...string) (claims, bool) {
	if token := r.Header.Get("X-Service-Token"); token != "" && s.config.ServiceToken != "" && token == s.config.ServiceToken {
		for _, role := range roles {
			if role == "Servicio" {
				return claims{Role: "Servicio", TokenType: "service"}, true
			}
		}
	}
	current, ok := s.authenticate(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Se requiere autenticación.", nil)
		return claims{}, false
	}
	if len(roles) == 0 {
		return current, true
	}
	for _, role := range roles {
		if current.Role == role {
			return current, true
		}
	}
	writeError(w, r, http.StatusForbidden, "FORBIDDEN", "El rol no tiene permiso para esta operación.", nil)
	return claims{}, false
}

func (s *Server) properties(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if _, ok := s.require(w, r, "Agente", "Administrador"); !ok {
			return
		}
		var input propertyRequest
		if !decodeJSON(w, r, &input) {
			return
		}
		if err := input.validate(); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		property, err := s.store.CreateProperty(r.Context(), input.toStore())
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusCreated, property)
		return
	}

	filter, err := propertyFilter(r)
	if err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	if _, ok := s.authenticate(r); !ok {
		filter.Status = "Disponible"
		filter.IncludeDeleted = false
	}
	properties, total, err := s.store.ListProperties(r.Context(), filter)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writePaginated(w, r, properties, filter.Page, filter.PageSize, total)
}

func (s *Server) property(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("inmuebleId")
	if !validID(id) {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "El inmueble no existe.", nil)
		return
	}
	switch r.Method {
	case http.MethodGet:
		_, authenticated := s.authenticate(r)
		property, err := s.store.GetProperty(r.Context(), id, authenticated)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, property)
	case http.MethodPatch:
		if _, ok := s.require(w, r, "Agente", "Administrador"); !ok {
			return
		}
		var fields map[string]interface{}
		if !decodeJSON(w, r, &fields) {
			return
		}
		property, err := s.store.UpdateProperty(r.Context(), id, fields)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, property)
	case http.MethodDelete:
		if _, ok := s.require(w, r, "Agente", "Administrador"); !ok {
			return
		}
		if err := s.store.DeleteProperty(r.Context(), id); err != nil {
			s.databaseError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) images(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("inmuebleId")
	if _, ok := s.requireIfWrite(w, r, "Agente", "Administrador"); !ok && r.Method != http.MethodGet {
		return
	}
	if r.Method == http.MethodGet {
		images, err := s.store.ListImages(r.Context(), id)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, images)
		return
	}
	var input struct {
		URL   string `json:"urlArchivo"`
		Order int    `json:"ordenVisualizacion"`
		Cover bool   `json:"esPortada"`
	}
	if !decodeJSON(w, r, &input) || input.URL == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "urlArchivo es obligatorio.", nil)
		return
	}
	image, err := s.store.AddImage(r.Context(), id, store.Image{FileURL: input.URL, DisplayOrder: input.Order, IsCover: input.Cover})
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, image)
}

func (s *Server) clients(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if _, ok := s.require(w, r, "Agente", "Administrador", "Servicio"); !ok {
			return
		}
		var input clientRequest
		if !decodeJSON(w, r, &input) {
			return
		}
		if err := input.validate(); err != nil {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
			return
		}
		hash, err := hashPassword(input.Password)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo proteger la contraseña.", nil)
			return
		}
		client, err := s.store.CreateClient(r.Context(), store.ClientInput{FirstName: input.FirstName, LastName: input.LastName, DNI: input.DNI, Email: input.Email, PasswordHash: hash, ConsentNotifications: input.ConsentNotifications, PrimaryClientType: input.PrimaryClientType})
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		for _, phone := range input.Phones {
			if _, err := s.store.CreatePhone(r.Context(), client.ID, phone.Number, phone.Primary); err != nil {
				s.databaseError(w, r, err)
				return
			}
		}
		writeData(w, r, http.StatusCreated, client)
		return
	}
	if _, ok := s.require(w, r, "Agente", "Administrador"); !ok {
		return
	}
	page, pageSize := pageParams(r)
	clients, total, err := s.store.ListClients(r.Context(), page, pageSize)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writePaginated(w, r, clients, page, pageSize, total)
}

func (s *Server) client(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r, "Agente", "Administrador", "Servicio")
	if !ok {
		return
	}
	id := r.PathValue("clienteId")
	switch r.Method {
	case http.MethodGet:
		client, err := s.store.GetClient(r.Context(), id)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, client)
	case http.MethodPatch:
		var fields map[string]interface{}
		if !decodeJSON(w, r, &fields) {
			return
		}
		client, err := s.store.UpdateClient(r.Context(), id, fields)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, client)
	case http.MethodDelete:
		if current.Role != "Administrador" {
			writeError(w, r, http.StatusForbidden, "FORBIDDEN", "Solo un administrador puede eliminar clientes.", nil)
			return
		}
		_, err := s.store.UpdateClient(r.Context(), id, map[string]interface{}{"estadoLead": "Inactivo"})
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		if _, err := s.store.DB.ExecContext(r.Context(), `UPDATE personas SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = $1`, id); err != nil {
			s.databaseError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) consent(w http.ResponseWriter, r *http.Request) {
	_, ok := s.require(w, r, "Agente", "Administrador", "Servicio")
	if !ok {
		return
	}
	id := r.PathValue("clienteId")
	var input struct {
		Consent *bool `json:"consentimientoNotificaciones"`
	}
	if !decodeJSON(w, r, &input) || input.Consent == nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "consentimientoNotificaciones es obligatorio.", nil)
		return
	}
	client, err := s.store.UpdateClient(r.Context(), id, map[string]interface{}{"consentimientoNotificaciones": *input.Consent})
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, client)
}

func (s *Server) phones(w http.ResponseWriter, r *http.Request) {
	_, ok := s.require(w, r, "Agente", "Administrador", "Servicio")
	if !ok {
		return
	}
	clientID := r.PathValue("clienteId")
	if r.Method == http.MethodGet {
		phones, err := s.store.ListPhones(r.Context(), clientID)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, phones)
		return
	}
	var input struct {
		Number  string `json:"numero"`
		Primary bool   `json:"esPrincipal"`
	}
	if !decodeJSON(w, r, &input) || input.Number == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "numero es obligatorio.", nil)
		return
	}
	phone, err := s.store.CreatePhone(r.Context(), clientID, input.Number, input.Primary)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, phone)
}

func (s *Server) phone(w http.ResponseWriter, r *http.Request) {
	_, ok := s.require(w, r, "Agente", "Administrador", "Servicio")
	if !ok {
		return
	}
	clientID := r.PathValue("clienteId")
	if err := s.store.DeletePhone(r.Context(), clientID, r.PathValue("telefonoId")); err != nil {
		s.databaseError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) preferences(w http.ResponseWriter, r *http.Request) {
	_, ok := s.require(w, r, "Agente", "Administrador", "Servicio")
	if !ok {
		return
	}
	clientID := r.PathValue("clienteId")
	if r.Method == http.MethodGet {
		preferences, err := s.store.ListPreferences(r.Context(), clientID)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, preferences)
		return
	}
	var input preferenceRequest
	if !decodeJSON(w, r, &input) {
		return
	}
	if err := input.validate(); err != nil {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error(), nil)
		return
	}
	preference, err := s.store.CreatePreference(r.Context(), clientID, input.toStore())
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, preference)
}

func (s *Server) inquiries(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		if _, ok := s.require(w, r, "Agente", "Administrador", "Servicio"); !ok {
			return
		}
		var input struct {
			ClientID   string  `json:"clienteId"`
			PropertyID string  `json:"inmuebleId"`
			Channel    string  `json:"canalOrigen"`
			Notes      *string `json:"notasInternas"`
		}
		if !decodeJSON(w, r, &input) || !validID(input.ClientID) || !validID(input.PropertyID) || input.Channel == "" {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "clienteId, inmuebleId y canalOrigen son obligatorios.", nil)
			return
		}
		inquiry, err := s.store.CreateInquiry(r.Context(), input.ClientID, input.PropertyID, input.Channel, input.Notes)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusCreated, inquiry)
		return
	}
	if _, ok := s.require(w, r, "Agente", "Administrador"); !ok {
		return
	}
	page, pageSize := pageParams(r)
	inquiries, total, err := s.store.ListInquiries(r.Context(), page, pageSize)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writePaginated(w, r, inquiries, page, pageSize, total)
}

func (s *Server) publicInquiry(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FirstName string `json:"nombre"`
		LastName  string `json:"apellido"`
		DNI       string `json:"dni"`
		Email     string `json:"correoElectronico"`
		Phone     string `json:"numeroTelefono"`
		Message   string `json:"mensaje"`
	}
	if !decodeJSON(w, r, &input) || input.FirstName == "" || input.LastName == "" || input.DNI == "" || input.Email == "" {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Los datos de contacto son obligatorios.", nil)
		return
	}
	client, err := s.store.GetClientByEmail(r.Context(), input.Email)
	created := false
	if errors.Is(err, store.ErrNotFound) {
		hash, hashErr := hashPassword("")
		if hashErr != nil {
			writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo registrar el cliente.", nil)
			return
		}
		client, err = s.store.CreateClient(r.Context(), store.ClientInput{FirstName: input.FirstName, LastName: input.LastName, DNI: input.DNI, Email: input.Email, PasswordHash: hash})
		created = err == nil
	}
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	if created && input.Phone != "" {
		if _, err := s.store.CreatePhone(r.Context(), client.ID, input.Phone, true); err != nil {
			s.databaseError(w, r, err)
			return
		}
	}
	var notes *string
	if input.Message != "" {
		notes = &input.Message
	}
	inquiry, err := s.store.CreateInquiry(r.Context(), client.ID, r.PathValue("inmuebleId"), "Portal Web", notes)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, inquiry)
}

func (s *Server) updateInquiry(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.require(w, r, "Agente", "Administrador"); !ok {
		return
	}
	var input struct {
		Status *string `json:"estadoSeguimiento"`
		Notes  *string `json:"notasInternas"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	inquiry, err := s.store.UpdateInquiry(r.Context(), r.PathValue("consultaId"), input.Status, input.Notes)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, inquiry)
}

func (s *Server) sessions(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeError(w, r, http.StatusNotImplemented, "NOT_IMPLEMENTED", "La lista paginada de sesiones aún no está implementada.", nil)
		return
	}
	if _, ok := s.require(w, r, "Agente", "Administrador", "Servicio"); !ok {
		return
	}
	var input struct {
		ClientID string `json:"clienteId"`
	}
	if !decodeJSON(w, r, &input) || !validID(input.ClientID) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "clienteId es obligatorio.", nil)
		return
	}
	session, err := s.store.CreateChatSession(r.Context(), input.ClientID)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusCreated, session)
}

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.require(w, r, "Agente", "Administrador", "Servicio"); !ok {
		return
	}
	session, err := s.store.GetChatSession(r.Context(), r.PathValue("sesionId"))
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, session)
}

func (s *Server) message(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.require(w, r, "Agente", "Administrador", "Servicio"); !ok {
		return
	}
	var message map[string]interface{}
	if !decodeJSON(w, r, &message) {
		return
	}
	if _, ok := message["contenido"].(string); !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "contenido es obligatorio.", nil)
		return
	}
	if _, ok := message["fechaHora"]; !ok {
		message["fechaHora"] = time.Now().UTC().Format(time.RFC3339)
	}
	if _, ok := message["id"]; !ok {
		message["id"] = randomID()
	}
	encoded, _ := json.Marshal(message)
	session, err := s.store.AppendMessage(r.Context(), r.PathValue("sesionId"), encoded)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, session)
}

func (s *Server) handoff(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, "Esperando Humano", "")
}
func (s *Server) resumeAI(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, "Atendida por IA", "")
}
func (s *Server) closeSession(w http.ResponseWriter, r *http.Request) {
	s.transition(w, r, "Cerrada", "")
}
func (s *Server) takeControl(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r, "Agente", "Administrador")
	if !ok {
		return
	}
	s.transitionWithAgent(w, r, "Atendida por Humano", current.Subject)
}
func (s *Server) transition(w http.ResponseWriter, r *http.Request, status, agent string) {
	if _, ok := s.require(w, r, "Agente", "Administrador", "Servicio"); !ok {
		return
	}
	s.transitionWithAgent(w, r, status, agent)
}
func (s *Server) transitionWithAgent(w http.ResponseWriter, r *http.Request, status, agent string) {
	session, err := s.store.TransitionChat(r.Context(), r.PathValue("sesionId"), status, agent)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, session)
}

func (s *Server) whatsappVerify(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("hub.verify_token") != s.config.ServiceToken {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(r.URL.Query().Get("hub.challenge")))
}

func (s *Server) whatsappWebhook(w http.ResponseWriter, r *http.Request) {
	if s.config.ServiceToken == "" || r.Header.Get("X-Webhook-Token") != s.config.ServiceToken {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Webhook no autorizado.", nil)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) notImplemented(w http.ResponseWriter, r *http.Request) {
	writeError(w, r, http.StatusNotImplemented, "NOT_IMPLEMENTED", "Este endpoint requiere tablas o componentes que todavía no están implementados.", nil)
}

func (s *Server) requireIfWrite(w http.ResponseWriter, r *http.Request, roles ...string) (claims, bool) {
	if r.Method == http.MethodGet {
		return claims{}, true
	}
	return s.require(w, r, roles...)
}

func (s *Server) databaseError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "El recurso no existe.", nil)
		return
	}
	if strings.Contains(err.Error(), "database is not configured") {
		writeError(w, r, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "La base de datos no está disponible.", nil)
		return
	}
	s.log.Error("database operation failed", "error", err, "request_id", requestID(r))
	writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo completar la operación.", nil)
}

type userRequest struct {
	FirstName string `json:"nombre"`
	LastName  string `json:"apellido"`
	DNI       string `json:"dni"`
	Email     string `json:"correoElectronico"`
	Password  string `json:"contrasena"`
	Role      string `json:"rol"`
}

func (u userRequest) validate() error {
	if u.FirstName == "" || u.LastName == "" || u.DNI == "" || u.Email == "" || u.Password == "" {
		return errors.New("nombre, apellido, dni, correoElectronico y contrasena son obligatorios")
	}
	if u.Role != "Agente" && u.Role != "Administrador" {
		return errors.New("rol debe ser Agente o Administrador")
	}
	return nil
}

type propertyRequest struct {
	AgentID         string                 `json:"agenteId"`
	Title           string                 `json:"titulo"`
	Description     *string                `json:"descripcion"`
	Operation       string                 `json:"tipoOperacion"`
	PropertyType    string                 `json:"tipoInmueble"`
	Status          string                 `json:"estado"`
	Price           float64                `json:"precio"`
	Currency        string                 `json:"moneda"`
	Expenses        *float64               `json:"montoExpensas"`
	Rooms           *int                   `json:"cantAmbientes"`
	Bedrooms        *int                   `json:"cantHabitaciones"`
	Bathrooms       *int                   `json:"cantBanos"`
	Garages         *int                   `json:"cantCocheras"`
	TotalArea       *float64               `json:"superficieTotal"`
	CoveredArea     *float64               `json:"superficieCubierta"`
	Street          *string                `json:"calle"`
	Number          *string                `json:"numero"`
	Neighborhood    *string                `json:"barrio"`
	City            *string                `json:"ciudad"`
	Latitude        *float64               `json:"latitud"`
	Longitude       *float64               `json:"longitud"`
	Characteristics map[string]interface{} `json:"caracteristicas"`
}

func (p propertyRequest) validate() error {
	if p.Title == "" || p.Operation == "" || p.PropertyType == "" || p.Currency == "" || p.Price <= 0 {
		return errors.New("titulo, tipoOperacion, tipoInmueble, precio y moneda son obligatorios y válidos")
	}
	return nil
}
func (p propertyRequest) toStore() store.PropertyInput {
	if p.Characteristics == nil {
		p.Characteristics = map[string]interface{}{}
	}
	if p.Status == "" {
		p.Status = "Disponible"
	}
	return store.PropertyInput{AgentID: p.AgentID, Title: p.Title, Description: p.Description, Operation: p.Operation, PropertyType: p.PropertyType, Status: p.Status, Price: p.Price, Currency: p.Currency, Expenses: p.Expenses, Rooms: p.Rooms, Bedrooms: p.Bedrooms, Bathrooms: p.Bathrooms, Garages: p.Garages, TotalArea: p.TotalArea, CoveredArea: p.CoveredArea, Street: p.Street, Number: p.Number, Neighborhood: p.Neighborhood, City: p.City, Latitude: p.Latitude, Longitude: p.Longitude, Characteristics: p.Characteristics}
}

type clientRequest struct {
	FirstName string `json:"nombre"`
	LastName  string `json:"apellido"`
	DNI       string `json:"dni"`
	Email     string `json:"correoElectronico"`
	Password  string `json:"contrasena"`
	Phones    []struct {
		Number  string `json:"numero"`
		Primary bool   `json:"esPrincipal"`
	} `json:"telefonos"`
	ConsentNotifications bool    `json:"consentimientoNotificaciones"`
	PrimaryClientType    *string `json:"tipoClientePrincipal"`
}

func (c clientRequest) validate() error {
	if c.FirstName == "" || c.LastName == "" || c.DNI == "" || c.Email == "" {
		return errors.New("nombre, apellido, dni y correoElectronico son obligatorios")
	}
	return nil
}

type preferenceRequest struct {
	MinPrice      *float64 `json:"rangoPrecioMin"`
	MaxPrice      *float64 `json:"rangoPrecioMax"`
	Currency      *string  `json:"moneda"`
	Operation     *string  `json:"tipoOperacion"`
	PropertyType  *string  `json:"tipoInmueble"`
	MinBedrooms   *int     `json:"cantidadDormitoriosMin"`
	PreferredArea *string  `json:"zonaPreferida"`
}

func (p preferenceRequest) validate() error {
	if p.MinPrice != nil && p.MaxPrice != nil && *p.MinPrice > *p.MaxPrice {
		return errors.New("rangoPrecioMin no puede superar rangoPrecioMax")
	}
	if (p.MinPrice != nil || p.MaxPrice != nil) && p.Currency == nil {
		return errors.New("moneda es obligatoria cuando se informa un rango de precio")
	}
	if p.MinBedrooms != nil && *p.MinBedrooms < 0 {
		return errors.New("cantidadDormitoriosMin no puede ser negativa")
	}
	if p.MinPrice == nil && p.MaxPrice == nil && p.Currency == nil && p.Operation == nil && p.PropertyType == nil && p.MinBedrooms == nil && p.PreferredArea == nil {
		return errors.New("debe informar al menos un criterio")
	}
	return nil
}
func (p preferenceRequest) toStore() store.PreferenceInput {
	return store.PreferenceInput{MinPrice: p.MinPrice, MaxPrice: p.MaxPrice, Currency: p.Currency, Operation: p.Operation, PropertyType: p.PropertyType, MinBedrooms: p.MinBedrooms, PreferredArea: p.PreferredArea}
}

func propertyFilter(r *http.Request) (store.PropertyFilter, error) {
	q := r.URL.Query()
	f := store.PropertyFilter{Operation: q.Get("tipoOperacion"), PropertyType: q.Get("tipoInmueble"), Status: q.Get("estado"), Currency: q.Get("moneda"), City: q.Get("ciudad"), Neighborhood: q.Get("barrio"), Query: q.Get("q"), AgentID: q.Get("agenteId")}
	f.Page, f.PageSize = pageParams(r)
	f.IncludeDeleted = q.Get("incluirEliminados") == "true"
	var err error
	f.MinPrice, err = optionalFloat(q.Get("precioMin"))
	if err != nil {
		return f, err
	}
	f.MaxPrice, err = optionalFloat(q.Get("precioMax"))
	if err != nil {
		return f, err
	}
	f.MinBedrooms, err = optionalInt(q.Get("cantHabitacionesMin"))
	if err != nil {
		return f, err
	}
	f.MinBathrooms, err = optionalInt(q.Get("cantBanosMin"))
	if err != nil {
		return f, err
	}
	f.MinTotalArea, err = optionalFloat(q.Get("superficieTotalMin"))
	if err != nil {
		return f, err
	}
	f.MaxTotalArea, err = optionalFloat(q.Get("superficieTotalMax"))
	return f, err
}
func pageParams(r *http.Request) (int, int) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}
func optionalFloat(value string) (*float64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("valor numérico inválido: %s", value)
	}
	return &parsed, nil
}
func optionalInt(value string) (*int, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil, fmt.Errorf("valor entero inválido: %s", value)
	}
	return &parsed, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, r, http.StatusBadRequest, "INVALID_REQUEST", "El cuerpo JSON no es válido.", nil)
		return false
	}
	return true
}
func hashPassword(password string) (string, error) {
	if password == "" {
		password = randomID() + randomID()
	}
	result, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(result), err
}
func validID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return false
		}
	}
	return true
}
func randomID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return hex.EncodeToString([]byte(time.Now().String()))[:32]
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s", hex.EncodeToString(bytes[0:4]), hex.EncodeToString(bytes[4:6]), hex.EncodeToString(bytes[6:8]), hex.EncodeToString(bytes[8:10]), hex.EncodeToString(bytes[10:16]))
}

func writeData(w http.ResponseWriter, r *http.Request, status int, data interface{}) {
	writeJSON(w, status, map[string]interface{}{"data": data, "meta": map[string]string{"requestId": requestID(r)}})
}
func writePaginated(w http.ResponseWriter, r *http.Request, data interface{}, page, pageSize, total int) {
	pages := 0
	if total > 0 {
		pages = (total + pageSize - 1) / pageSize
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": data, "meta": map[string]interface{}{"page": page, "pageSize": pageSize, "total": total, "totalPages": pages, "requestId": requestID(r)}})
}
func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, details interface{}) {
	writeJSON(w, status, map[string]interface{}{"error": map[string]interface{}{"code": code, "message": message, "details": details, "requestId": requestID(r)}})
}
func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func requestID(r *http.Request) string {
	if id, ok := r.Context().Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !validID(id) {
			id = randomID()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Ocurrió un error interno.", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
func loggingMiddleware(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("http request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started), "request_id", requestID(r))
	})
}
