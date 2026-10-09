// Package api — HTTP JSON API сервиса.
package api

import (
	"context"
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
	mux.HandleFunc("PATCH /api/sessions/{id}", s.updateSession)
	mux.HandleFunc("DELETE /api/sessions/{id}", s.deleteSession)
	mux.HandleFunc("PUT /api/sessions/{id}/order", s.reorderMatches)
	mux.HandleFunc("POST /api/sessions/{id}/demos", s.uploadDemos)
	mux.HandleFunc("GET /api/matches/{id}", s.getMatch)
	mux.HandleFunc("DELETE /api/matches/{id}", s.deleteMatch)
	mux.HandleFunc("POST /api/matches/{id}/reparse", s.reparseMatch)
	mux.HandleFunc("POST /api/sessions/{id}/reparse", s.reparseSession)
	mux.HandleFunc("GET /api/players", s.listPlayers)
	mux.HandleFunc("GET /api/players/{steamId}", s.getPlayer)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "неизвестный метод API")
	})
	if static != nil {
		mux.Handle("/", static)
	}
	return mux
}

// bestPlayer — лучший игрок сессии в списке сессий.
type bestPlayer struct {
	SteamID string  `json:"steamId"`
	Name    string  `json:"name"`
	Rating  float64 `json:"rating"`
}

type sessionSummaryView struct {
	store.SessionSummary
	Best *bestPlayer `json:"best,omitempty"`
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListSessions(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	views := make([]sessionSummaryView, 0, len(list))
	for _, x := range list {
		v := sessionSummaryView{SessionSummary: x}
		if x.MatchCount > 0 {
			agg, err := s.Store.SessionPlayers(r.Context(), x.ID)
			if err != nil {
				s.internalError(w, err)
				return
			}
			v.Best = pickBest(totalsView(agg))
		}
		views = append(views, v)
	}
	writeJSON(w, http.StatusOK, views)
}

// pickBest выбирает игрока с наибольшим rating, при равенстве — с большим числом раундов, затем по нику.
func pickBest(players []PlayerView) *bestPlayer {
	var best *PlayerView
	for i := range players {
		p := &players[i]
		if best == nil || p.Rating > best.Rating ||
			p.Rating == best.Rating && (p.Rounds > best.Rounds || p.Rounds == best.Rounds && p.Name < best.Name) {
			best = p
		}
	}
	if best == nil {
		return nil
	}
	return &bestPlayer{SteamID: best.SteamID, Name: best.Name, Rating: best.Rating}
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
	if req.Date == "" {
		req.Date = s.now().Format(time.DateOnly)
	}
	if msg := req.normalize(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	sess, err := s.Store.CreateSession(r.Context(), req.Date, req.Title)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

// normalize обрезает пробелы в названии и проверяет поля. Возвращает текст ошибки или "".
func (req *createSessionRequest) normalize() string {
	req.Title = strings.TrimSpace(req.Title)
	if utf8.RuneCountInString(req.Title) > maxTitleLen {
		return "название длиннее 200 символов"
	}
	if _, err := time.Parse(time.DateOnly, req.Date); err != nil {
		return "некорректная дата, ожидается ГГГГ-ММ-ДД"
	}
	return ""
}

// updateSession меняет дату и название. В отличие от создания, дата обязательна:
// подставлять «сегодня» при правке было бы неожиданно.
func (s *Server) updateSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req createSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	if msg := req.normalize(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	sess, err := s.Store.UpdateSession(r.Context(), id, req.Date, req.Title)
	if err != nil {
		s.storeError(w, err, "сессия не найдена")
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) deleteSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	shas, err := s.Store.DeleteSession(r.Context(), id)
	if err != nil {
		s.storeError(w, err, "сессия не найдена")
		return
	}
	s.removeDemos(r, shas)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deleteMatch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	m, err := s.Store.DeleteMatch(r.Context(), id)
	if err != nil {
		s.storeError(w, err, "матч не найден")
		return
	}
	s.removeDemos(r, []string{m.SHA256})
	w.WriteHeader(http.StatusNoContent)
}

// removeDemos удаляет файлы демок после коммита удаления. Данные в БД уже удалены,
// поэтому ошибка только пишется в лог: файл-сирота безвреден.
func (s *Server) removeDemos(r *http.Request, shas []string) {
	if len(shas) == 0 {
		return
	}
	if err := s.Ingest.RemoveDemos(context.WithoutCancel(r.Context()), shas); err != nil {
		s.Log.Warn("файлы демок не удалены", "err", err)
	}
}

type reorderRequest struct {
	MatchIDs []int64 `json:"matchIds"`
}

func (s *Server) reorderMatches(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req reorderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}
	matches, err := s.Store.ReorderMatches(r.Context(), id, req.MatchIDs)
	if errors.Is(err, store.ErrOrderMismatch) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		s.storeError(w, err, "сессия не найдена")
		return
	}
	writeJSON(w, http.StatusOK, matches)
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
	writeJSON(w, http.StatusOK, sessionResponse{Session: sess, Matches: matches, Players: totalsView(agg)})
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
	if accepted {
		s.wake()
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

func (s *Server) reparseMatch(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	m, err := s.Store.RequeueMatch(r.Context(), id)
	if err != nil {
		s.storeError(w, err, "матч не найден")
		return
	}
	s.wake()
	writeJSON(w, http.StatusAccepted, m)
}

func (s *Server) reparseSession(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if _, err := s.Store.GetSession(r.Context(), id); err != nil {
		s.storeError(w, err, "сессия не найдена")
		return
	}
	n, err := s.Store.RequeueSession(r.Context(), id)
	if err != nil {
		s.internalError(w, err)
		return
	}
	if n > 0 {
		s.wake()
	}
	writeJSON(w, http.StatusAccepted, map[string]int64{"queued": n})
}

func (s *Server) wake() {
	if s.Wake != nil {
		s.Wake()
	}
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
