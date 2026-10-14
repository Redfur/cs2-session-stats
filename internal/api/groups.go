package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"cs2stats/internal/store"
)

const maxGroupNameLen = 100

// groupFromPath разбирает {gid} и проверяет, что группа существует. ok=false — ответ уже записан.
func (s *Server) groupFromPath(w http.ResponseWriter, r *http.Request) (int64, bool) {
	gid, err := strconv.ParseInt(r.PathValue("gid"), 10, 64)
	if err != nil || gid <= 0 {
		writeError(w, http.StatusNotFound, "группа не найдена")
		return 0, false
	}
	if _, err := s.Store.GetGroup(r.Context(), gid); err != nil {
		s.storeError(w, err, "группа не найдена")
		return 0, false
	}
	return gid, true
}

type groupRequest struct {
	Name string `json:"name"`
}

// decodeGroupRequest читает и проверяет название группы. ok=false — ответ уже записан.
func decodeGroupRequest(w http.ResponseWriter, r *http.Request) (string, bool) {
	var req groupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return "", false
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "введите название группы")
		return "", false
	}
	if utf8.RuneCountInString(name) > maxGroupNameLen {
		writeError(w, http.StatusBadRequest, "название группы длиннее 100 символов")
		return "", false
	}
	return name, true
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListGroups(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	name, ok := decodeGroupRequest(w, r)
	if !ok {
		return
	}
	g, err := s.Store.CreateGroup(r.Context(), name)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, g)
}

func (s *Server) getGroup(w http.ResponseWriter, r *http.Request) {
	gid, ok := s.groupFromPath(w, r)
	if !ok {
		return
	}
	g, err := s.Store.GroupSummary(r.Context(), gid)
	if err != nil {
		s.storeError(w, err, "группа не найдена")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) renameGroup(w http.ResponseWriter, r *http.Request) {
	gid, ok := s.groupFromPath(w, r)
	if !ok {
		return
	}
	name, ok := decodeGroupRequest(w, r)
	if !ok {
		return
	}
	g, err := s.Store.RenameGroup(r.Context(), gid, name)
	if err != nil {
		s.storeError(w, err, "группа не найдена")
		return
	}
	writeJSON(w, http.StatusOK, g)
}

func (s *Server) deleteGroup(w http.ResponseWriter, r *http.Request) {
	gid, ok := s.groupFromPath(w, r)
	if !ok {
		return
	}
	err := s.Store.DeleteGroup(r.Context(), gid)
	if errors.Is(err, store.ErrGroupNotEmpty) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		s.storeError(w, err, "группа не найдена")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
