// Package api — HTTP JSON API сервиса.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"cs2stats/internal/ingest"
	"cs2stats/internal/store"
)

const maxTitleLen = 200

type Server struct {
	Store  *store.Store
	Ingest *ingest.Service
	Wake   func() // будит воркер после загрузки демок
	Log    *slog.Logger
	Now    func() time.Time
}

// Handler возвращает маршруты API; остальные пути обслуживает static (фронтенд).
func (s *Server) Handler(static http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/sessions", s.listSessions)
	mux.HandleFunc("POST /api/sessions", s.createSession)
	mux.HandleFunc("GET /api/sessions/{id}", s.getSession)
	mux.HandleFunc("POST /api/sessions/{id}/demos", s.uploadDemos)
	mux.HandleFunc("GET /api/matches/{id}", s.getMatch)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "неизвестный метод API")
	})
	if static != nil {
		mux.Handle("/", static)
	}
	return mux
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListSessions(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type createSessionRequest struct {
	Date  string `json:"date"`
	Title string `json:"title"`
}

func (s *Server) createSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if utf8.RuneCountInString(req.Title) > maxTitleLen {
		writeError(w, http.StatusBadRequest, "название длиннее 200 символов")
		return
	}
	if req.Date == "" {
		req.Date = s.now().Format(time.DateOnly)
	} else if _, err := time.Parse(time.DateOnly, req.Date); err != nil {
		writeError(w, http.StatusBadRequest, "некорректная дата, ожидается ГГГГ-ММ-ДД")
		return
	}
	sess, err := s.Store.CreateSession(r.Context(), req.Date, req.Title)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

type sessionResponse struct {
	Session store.Session `json:"session"`
	Matches []store.Match `json:"matches"`
	Players []PlayerView  `json:"players"`
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	sess, err := s.Store.GetSession(r.Context(), id)
	if err != nil {
		s.storeError(w, err, "сессия не найдена")
		return
	}
	matches, err := s.Store.ListSessionMatches(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	agg, err := s.Store.SessionPlayers(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	players := make([]PlayerView, 0, len(agg))
	for _, p := range agg {
		v := newPlayerView(p.SteamID, p.Name, p.Counters)
		v.Matches, v.Wins = p.Matches, p.Wins
		players = append(players, v)
	}
	sort.SliceStable(players, func(i, j int) bool { return players[i].Rating > players[j].Rating })
	writeJSON(w, http.StatusOK, sessionResponse{Session: sess, Matches: matches, Players: players})
}

func (s *Server) uploadDemos(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.Store.GetSession(r.Context(), id); err != nil {
		s.storeError(w, err, "сессия не найдена")
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "ожидается multipart/form-data с файлами в поле files")
		return
	}
	results := []ingest.FileResult{}
	accepted := false
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "ошибка чтения загрузки: "+err.Error())
			return
		}
		if part.FormName() != "files" || part.FileName() == "" {
			part.Close()
			continue
		}
		res := s.Ingest.Ingest(r.Context(), id, part.FileName(), part)
		part.Close()
		if res.Status == ingest.Accepted {
			accepted = true
		}
		results = append(results, res)
	}
	if accepted && s.Wake != nil {
		s.Wake()
	}
	if len(results) == 0 {
		writeError(w, http.StatusBadRequest, "не выбрано ни одного файла")
		return
	}
	writeJSON(w, http.StatusOK, results)
}

type matchResponse struct {
	Match   store.Match  `json:"match"`
	Players []PlayerView `json:"players"`
}

func (s *Server) getMatch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	m, err := s.Store.GetMatch(r.Context(), id)
	if err != nil {
		s.storeError(w, err, "матч не найден")
		return
	}
	ps, err := s.Store.MatchPlayers(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	players := make([]PlayerView, 0, len(ps))
	for _, p := range ps {
		players = append(players, MatchPlayerView(p))
	}
	sort.SliceStable(players, func(i, j int) bool {
		if players[i].Team != players[j].Team {
			return players[i].Team < players[j].Team
		}
		return players[i].Rating > players[j].Rating
	})
	writeJSON(w, http.StatusOK, matchResponse{Match: m, Players: players})
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusNotFound, "не найдено")
		return 0, false
	}
	return id, true
}

func (s *Server) storeError(w http.ResponseWriter, err error, notFoundMsg string) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, notFoundMsg)
		return
	}
	s.internalError(w, err)
}

func (s *Server) internalError(w http.ResponseWriter, err error) {
	s.Log.Error("ошибка API", "err", err)
	writeError(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
