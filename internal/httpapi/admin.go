package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"
)

var inquiryStatuses = map[string]bool{
	"Pendiente": true, "Contactado": true, "Visita Agendada": true, "Descartado": true, "Cerrado": true,
}

func (s *Server) agents(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.require(w, r, "Administrador"); !ok {
		return
	}
	users, err := s.store.ListAgents(r.Context())
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, users)
}

func (s *Server) restoreProperty(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r, "Agente", "Administrador")
	if !ok {
		return
	}
	id := r.PathValue("inmuebleId")
	if !validID(id) {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "El inmueble no existe.", nil)
		return
	}
	if !s.mayManageProperty(w, r, current, id) {
		return
	}
	property, err := s.store.RestoreProperty(r.Context(), id)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, property)
}

func (s *Server) image(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r, "Agente", "Administrador")
	if !ok {
		return
	}
	propertyID := r.PathValue("inmuebleId")
	imageID := r.PathValue("imagenId")
	if !validID(propertyID) || !validID(imageID) {
		writeError(w, r, http.StatusNotFound, "NOT_FOUND", "La imagen no existe.", nil)
		return
	}
	if !s.mayManageProperty(w, r, current, propertyID) {
		return
	}
	switch r.Method {
	case http.MethodPatch:
		var input struct {
			Order *int  `json:"ordenVisualizacion"`
			Cover *bool `json:"esPortada"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		if input.Order != nil && (*input.Order < 1 || *input.Order > 999) {
			writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "ordenVisualizacion debe estar entre 1 y 999.", nil)
			return
		}
		image, err := s.store.UpdateImage(r.Context(), propertyID, imageID, input.Order, input.Cover)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		writeData(w, r, http.StatusOK, image)
	case http.MethodDelete:
		image, err := s.store.DeleteImage(r.Context(), propertyID, imageID)
		if err != nil {
			s.databaseError(w, r, err)
			return
		}
		if image.StorageKey != nil {
			if err := s.r2.Delete(r.Context(), *image.StorageKey); err != nil {
				s.log.Error("delete r2 image", "error", err, "key", *image.StorageKey)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (s *Server) presignUpload(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r, "Agente", "Administrador")
	if !ok {
		return
	}
	var input struct {
		PropertyID  string `json:"inmuebleId"`
		ContentType string `json:"contentType"`
	}
	if !decodeJSON(w, r, &input) || !validID(input.PropertyID) {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "inmuebleId es obligatorio.", nil)
		return
	}
	extension, ok := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp"}[input.ContentType]
	if !ok {
		writeError(w, r, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "contentType debe ser image/jpeg, image/png o image/webp.", nil)
		return
	}
	if !s.r2.Enabled() {
		writeError(w, r, http.StatusServiceUnavailable, "R2_UNAVAILABLE", "La configuración de imágenes no está disponible.", nil)
		return
	}
	if !s.mayManageProperty(w, r, current, input.PropertyID) {
		return
	}
	key := "inmuebles/" + input.PropertyID + "/" + randomID() + "." + extension
	uploadURL, err := s.r2.PresignPut(r.Context(), key, input.ContentType, s.config.R2PresignTTL)
	if err != nil {
		s.log.Error("presign upload", "error", err)
		writeError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "No se pudo preparar la subida.", nil)
		return
	}
	writeData(w, r, http.StatusOK, map[string]interface{}{
		"uploadUrl":  uploadURL,
		"storageKey": key,
		"publicUrl":  s.r2.PublicURL(key),
		"expiresIn":  int(s.config.R2PresignTTL.Seconds()),
		"maxBytes":   s.config.R2MaxUpload,
	})
}

func (s *Server) inquiry(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r, "Agente", "Administrador")
	if !ok {
		return
	}
	item, err := s.store.GetInquiryDetail(r.Context(), r.PathValue("consultaId"))
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	if current.Role == "Agente" && (item.AssignedAgent == nil || *item.AssignedAgent != current.Subject) {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "La consulta no está asignada a su cartera.", nil)
		return
	}
	writeData(w, r, http.StatusOK, item)
}

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	current, ok := s.require(w, r, "Agente", "Administrador")
	if !ok {
		return
	}
	agentID := ""
	if current.Role == "Agente" {
		agentID = current.Subject
	}
	data, err := s.store.Dashboard(r.Context(), agentID)
	if err != nil {
		s.databaseError(w, r, err)
		return
	}
	writeData(w, r, http.StatusOK, data)
}

func (s *Server) mayManageProperty(w http.ResponseWriter, r *http.Request, current claims, propertyID string) bool {
	if current.Role == "Administrador" {
		return true
	}
	owner, err := s.store.PropertyOwner(r.Context(), propertyID)
	if err != nil {
		s.databaseError(w, r, err)
		return false
	}
	if owner != current.Subject {
		writeError(w, r, http.StatusForbidden, "FORBIDDEN", "El inmueble no pertenece a su cartera.", nil)
		return false
	}
	return true
}

func validateInquiryStatus(status *string) error {
	if status != nil && !inquiryStatuses[*status] {
		return errors.New("estadoSeguimiento no es válido")
	}
	return nil
}

func normalizeText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func expiresIn(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
