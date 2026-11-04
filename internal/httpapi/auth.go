package httpapi

import (
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 7 * 24 * time.Hour
	refreshCookie   = "adelson_refresh"
)

type attemptLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	max      int
	window   time.Duration
}

func newAttemptLimiter(max int, window time.Duration) *attemptLimiter {
	return &attemptLimiter{attempts: map[string][]time.Time{}, max: max, window: window}
}

func (l *attemptLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-l.window)
	recent := l.attempts[key][:0]
	for _, attempt := range l.attempts[key] {
		if attempt.After(cutoff) {
			recent = append(recent, attempt)
		}
	}
	l.attempts[key] = recent
	return len(recent) < l.max
}

func (l *attemptLimiter) add(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts[key] = append(l.attempts[key], time.Now())
}

func (l *attemptLimiter) reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

var dummyPasswordHash = sync.OnceValue(func() []byte {
	hash, _ := bcrypt.GenerateFromPassword([]byte("adelson-login-timing-equalizer"), bcrypt.DefaultCost)
	return hash
})

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
	email := strings.ToLower(strings.TrimSpace(input.Email))
	ipKey := clientIP(r) + "|" + email
	accountKey := "account|" + email
	if !s.limiter.allow(ipKey) || !s.accountLimiter.allow(accountKey) {
		w.Header().Set("Retry-After", "900")
		writeError(w, r, http.StatusTooManyRequests, "TOO_MANY_ATTEMPTS", "Demasiados intentos. Espere unos minutos.", nil)
		return
	}

	user, err := s.store.GetAuthUserByEmail(r.Context(), email)
	passwordOK := false
	if err == nil {
		passwordOK = bcrypt.CompareHashAndPassword([]byte(user.Hash), []byte(input.Password)) == nil
	} else {
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(input.Password))
	}
	if err != nil || !passwordOK || user.Operational != "Activo" {
		s.limiter.add(ipKey)
		s.accountLimiter.add(accountKey)
		s.log.Warn("login failed", "email", email, "ip", clientIP(r))
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Las credenciales no son válidas.", nil)
		return
	}
	s.limiter.reset(ipKey)
	s.accountLimiter.reset(accountKey)
	w.Header().Set("Cache-Control", "no-store")
	if _, err := s.store.DB.ExecContext(r.Context(), `UPDATE usuarios SET ultimo_acceso = CURRENT_TIMESTAMP WHERE id = $1`, user.ID); err != nil {
		s.log.Error("update last access", "error", err)
	}
	profile, err := s.store.GetUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo cargar el usuario.", nil)
		return
	}
	profile.LastAccess = timePtr(time.Now())
	access, _, err := s.issueToken(user.ID, user.Role, "access", accessTokenTTL)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo crear la sesión.", nil)
		return
	}
	refresh, jti, err := s.issueToken(user.ID, user.Role, "refresh", refreshTokenTTL)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo crear la sesión.", nil)
		return
	}
	if err := s.store.CreateRefreshSession(r.Context(), user.ID, jti, r.UserAgent(), clientIP(r), time.Now().Add(refreshTokenTTL)); err != nil {
		s.log.Error("create refresh session", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo crear la sesión.", nil)
		return
	}
	s.setRefreshCookie(w, refresh, int(refreshTokenTTL.Seconds()))
	writeData(w, r, http.StatusOK, map[string]interface{}{
		"accessToken": access,
		"tokenType":   "Bearer",
		"expiresIn":   int(accessTokenTTL.Seconds()),
		"usuario":     profile,
	})
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(refreshCookie)
	if err != nil {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Se requiere un refresh token válido.", nil)
		return
	}
	current, ok := s.parseToken(cookie.Value, "refresh")
	if !ok || current.ID == "" {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Se requiere un refresh token válido.", nil)
		return
	}
	valid, err := s.store.RefreshSessionValid(r.Context(), current.Subject, current.ID)
	if err != nil || !valid {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "La sesión expiró. Inicie sesión nuevamente.", nil)
		return
	}
	user, err := s.store.GetAuthUser(r.Context(), current.Subject)
	if err != nil || user.Operational != "Activo" {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "La sesión expiró. Inicie sesión nuevamente.", nil)
		return
	}
	access, _, err := s.issueToken(user.ID, user.Role, "access", accessTokenTTL)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo renovar la sesión.", nil)
		return
	}
	newRefresh, newJTI, err := s.issueToken(user.ID, user.Role, "refresh", refreshTokenTTL)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo renovar la sesión.", nil)
		return
	}
	if err := s.store.RotateRefreshSession(r.Context(), current.ID, newJTI, user.ID, r.UserAgent(), clientIP(r), time.Now().Add(refreshTokenTTL)); err != nil {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "La sesión expiró. Inicie sesión nuevamente.", nil)
		return
	}
	s.setRefreshCookie(w, newRefresh, int(refreshTokenTTL.Seconds()))
	w.Header().Set("Cache-Control", "no-store")
	writeData(w, r, http.StatusOK, map[string]interface{}{"accessToken": access, "tokenType": "Bearer", "expiresIn": int(accessTokenTTL.Seconds())})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(refreshCookie); err == nil {
		if current, ok := s.parseToken(cookie.Value, "refresh"); ok && current.ID != "" {
			if err := s.store.RevokeRefreshSession(r.Context(), current.ID); err != nil {
				s.log.Error("revoke refresh session", "error", err)
			}
		}
	}
	s.setRefreshCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r)
	if !ok {
		return
	}
	user, err := s.store.GetUser(r.Context(), current.Subject)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, user)
}

func (s *Server) issueToken(subject, role, tokenType string, duration time.Duration) (string, string, error) {
	now := time.Now()
	jti := randomID()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{Role: role, TokenType: tokenType, RegisteredClaims: jwt.RegisteredClaims{
		Subject: subject, Issuer: "adelson-api", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(duration)), ID: jti,
	}})
	signed, err := token.SignedString([]byte(s.config.JWTSecret))
	return signed, jti, err
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
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("adelson-api"))
	if err != nil || !token.Valid {
		return claims{}, false
	}
	parsed, ok := token.Claims.(*claims)
	if !ok || parsed.TokenType != expectedType || parsed.Subject == "" {
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
	user, err := s.store.GetAuthUser(r.Context(), current.Subject)
	if err != nil || user.Operational != "Activo" {
		writeError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "La cuenta no está activa o no existe.", nil)
		return claims{}, false
	}
	current.Role = user.Role
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

func (s *Server) requireActive(w http.ResponseWriter, r *http.Request) (claims, bool) {
	current, ok := s.authenticate(r)
	if !ok {
		return claims{}, false
	}
	user, err := s.store.GetAuthUser(r.Context(), current.Subject)
	if err != nil || user.Operational != "Activo" {
		return claims{}, false
	}
	current.Role = user.Role
	return current, true
}

func (s *Server) setRefreshCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: refreshCookie, Value: value, Path: "/api/v1/auth", MaxAge: maxAge, HttpOnly: true, Secure: s.config.CookieSecure, SameSite: http.SameSiteLaxMode})
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first := strings.TrimSpace(strings.Split(forwarded, ",")[0])
		if net.ParseIP(first) != nil {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func timePtr(value time.Time) *time.Time {
	return &value
}
