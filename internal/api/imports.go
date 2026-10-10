package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"cs2stats/internal/importer"
	"cs2stats/internal/store"
)

// Imports — проверка ссылок на матчи и фоновое скачивание демок (importer.Importer).
type Imports interface {
	Add(ctx context.Context, sessionID int64, text string) ([]importer.AddResult, error)
	Wake()
	Cancel(importID int64)
	CancelSession(sessionID int64)
}

type addImportsRequest struct {
	Text string `json:"text"`
}

// addImports — POST /api/sessions/{id}/imports: проверить ссылки и поставить матчи в очередь скачивания.
func (s *Server) addImports(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req addImportsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if _, err := s.Store.GetSession(r.Context(), id); err != nil {
		s.storeError(w, err, "сессия не найдена")
		return
	}
	if s.Imports == nil {
		writeError(w, http.StatusServiceUnavailable, "импорт по ссылке недоступен")
		return
	}
	results, err := s.Imports.Add(r.Context(), id, req.Text)
	if errors.Is(err, importer.ErrEmptyInput) || errors.Is(err, importer.ErrTooMany) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	for _, res := range results {
		if res.Status == importer.AddAccepted || res.Status == importer.AddRetried {
			s.Imports.Wake()
			break
		}
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) retryImport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	x, err := s.Store.RetryImport(r.Context(), id)
	if errors.Is(err, store.ErrImportState) {
		writeError(w, http.StatusConflict, "повторить можно только загрузку с ошибкой")
		return
	}
	if err != nil {
		s.storeError(w, err, "загрузка не найдена")
		return
	}
	if s.Imports != nil {
		s.Imports.Wake()
	}
	writeJSON(w, http.StatusAccepted, x)
}

// deleteImport убирает загрузку из списка и прерывает её скачивание; созданные матчи остаются.
func (s *Server) deleteImport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.Store.DeleteImport(r.Context(), id); err != nil {
		s.storeError(w, err, "загрузка не найдена")
		return
	}
	if s.Imports != nil {
		s.Imports.Cancel(id)
	}
	w.WriteHeader(http.StatusNoContent)
}
