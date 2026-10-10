package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type ImportStatus string

const (
	ImportQueued      ImportStatus = "queued"
	ImportDownloading ImportStatus = "downloading"
	ImportDone        ImportStatus = "done"
	ImportFailed      ImportStatus = "failed"
)

// ErrImportState — действие недоступно в текущем статусе загрузки (например, повтор не упавшей загрузки).
var ErrImportState = errors.New("действие недоступно в текущем статусе загрузки")

// Import — загрузка по ссылке на матч платформы.
type Import struct {
	ID         int64        `json:"id"`
	SessionID  int64        `json:"sessionId"`
	URL        string       `json:"url"`
	Platform   string       `json:"platform"`
	ExternalID string       `json:"externalId"`
	Status     ImportStatus `json:"status"`
	Error      string       `json:"error,omitempty"`
	BytesDone  int64        `json:"bytesDone"`
	BytesTotal *int64       `json:"bytesTotal,omitempty"`
	// Results — итог по каждой карте в формате ingest.FileResult; store его не разбирает.
	Results    json.RawMessage `json:"results"`
	CreatedAt  string          `json:"createdAt"`
	FinishedAt *string         `json:"finishedAt,omitempty"`
}

const importColumns = `id, session_id, url, platform, external_id, status, error, bytes_done, bytes_total,
	results, created_at, finished_at`

func scanImport(row scanner) (Import, error) {
	var x Import
	var total sql.NullInt64
	var finished sql.NullString
	var results string
	err := row.Scan(&x.ID, &x.SessionID, &x.URL, &x.Platform, &x.ExternalID, &x.Status, &x.Error,
		&x.BytesDone, &total, &results, &x.CreatedAt, &finished)
	if total.Valid {
		x.BytesTotal = &total.Int64
	}
	if finished.Valid {
		x.FinishedAt = &finished.String
	}
	x.Results = json.RawMessage(results)
	return x, err
}

func (s *Store) queryImport(ctx context.Context, q string, args ...any) (Import, error) {
	x, err := scanImport(s.db.QueryRowContext(ctx, q, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return Import{}, ErrNotFound
	}
	return x, err
}

// AddImport ставит загрузку в очередь. Сессия должна существовать.
func (s *Store) AddImport(ctx context.Context, sessionID int64, url, platform, externalID string) (Import, error) {
	return s.queryImport(ctx, `
		INSERT INTO imports (session_id, url, platform, external_id, created_at)
		VALUES (?, ?, ?, ?, ?)
		RETURNING `+importColumns,
		sessionID, url, platform, externalID, s.timestamp())
}

func (s *Store) GetImport(ctx context.Context, id int64) (Import, error) {
	return s.queryImport(ctx, "SELECT "+importColumns+" FROM imports WHERE id = ?", id)
}

// ListSessionImports возвращает загрузки сессии, которые показываются на её странице:
// в работе, упавшие и завершённые, у которых не все карты стали новыми матчами.
func (s *Store) ListSessionImports(ctx context.Context, sessionID int64) ([]Import, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT `+importColumns+` FROM imports
		WHERE session_id = ? AND (status <> 'done' OR EXISTS (
			SELECT 1 FROM json_each(imports.results) WHERE json_extract(value, '$.status') <> 'accepted'))
		ORDER BY id`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Import{}
	for rows.Next() {
		x, err := scanImport(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, x)
	}
	return list, rows.Err()
}

// ClaimNextImport переводит самую раннюю загрузку из очереди в downloading. ok=false, если очередь пуста.
func (s *Store) ClaimNextImport(ctx context.Context) (Import, bool, error) {
	x, err := s.queryImport(ctx, `
		UPDATE imports SET status = 'downloading', error = '', bytes_done = 0, bytes_total = NULL
		WHERE id = (SELECT id FROM imports WHERE status = 'queued' ORDER BY id LIMIT 1)
		RETURNING `+importColumns)
	if errors.Is(err, ErrNotFound) {
		return Import{}, false, nil
	}
	return x, err == nil, err
}

// ResetDownloading возвращает в очередь загрузки, прерванные остановкой сервиса.
func (s *Store) ResetDownloading(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		"UPDATE imports SET status = 'queued', bytes_done = 0, bytes_total = NULL WHERE status = 'downloading'")
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdateImportProgress сохраняет прогресс скачивания; total = nil, если размер неизвестен.
func (s *Store) UpdateImportProgress(ctx context.Context, id, done int64, total *int64) error {
	return s.execOne(ctx, "UPDATE imports SET bytes_done = ?, bytes_total = ? WHERE id = ?", done, total, id)
}

// FinishImport завершает загрузку: done без ошибки или failed с текстом. results — итог по картам.
// ErrNotFound, если загрузку удалили во время скачивания.
func (s *Store) FinishImport(ctx context.Context, id int64, errMsg string, results json.RawMessage) error {
	status := ImportDone
	if errMsg != "" {
		status = ImportFailed
	}
	if len(results) == 0 || string(results) == "null" {
		results = json.RawMessage("[]")
	}
	return s.execOne(ctx,
		"UPDATE imports SET status = ?, error = ?, results = ?, finished_at = ? WHERE id = ?",
		status, errMsg, string(results), s.timestamp(), id)
}

// RetryImport ставит упавшую загрузку в очередь заново.
func (s *Store) RetryImport(ctx context.Context, id int64) (Import, error) {
	x, err := s.queryImport(ctx, `
		UPDATE imports SET status = 'queued', error = '', bytes_done = 0, bytes_total = NULL,
			results = '[]', finished_at = NULL
		WHERE id = ? AND status = 'failed'
		RETURNING `+importColumns, id)
	if errors.Is(err, ErrNotFound) {
		if _, err := s.GetImport(ctx, id); err != nil {
			return Import{}, err
		}
		return Import{}, ErrImportState
	}
	return x, err
}

// DeleteImport удаляет загрузку; её матчи остаются, import_id у них обнуляется.
func (s *Store) DeleteImport(ctx context.Context, id int64) (Import, error) {
	return s.queryImport(ctx, "DELETE FROM imports WHERE id = ? RETURNING "+importColumns, id)
}

// ImportSource — что уже есть по матчу платформы перед добавлением ссылки.
type ImportSource struct {
	Active *Import // загрузка в очереди или в работе, в любой сессии
	Match  *Match  // матч из этого источника: сначала из этой сессии, иначе любой
	Failed *Import // упавшая загрузка этого источника в этой сессии
}

func (s *Store) FindImportSource(ctx context.Context, sessionID int64, platform, externalID string, number int64) (ImportSource, error) {
	var src ImportSource
	x, err := s.queryImport(ctx, `
		SELECT `+importColumns+` FROM imports
		WHERE platform = ? AND external_id = ? AND status IN ('queued', 'downloading')
		ORDER BY id LIMIT 1`, platform, externalID)
	switch {
	case err == nil:
		src.Active = &x
	case !errors.Is(err, ErrNotFound):
		return src, err
	}

	m, err := scanMatch(s.db.QueryRowContext(ctx, `
		SELECT `+matchColumns+` FROM matches
		WHERE source_platform = ? AND source_number = ?
		ORDER BY session_id = ? DESC, session_id, ordinal LIMIT 1`, platform, number, sessionID))
	switch {
	case err == nil:
		src.Match = &m
	case !errors.Is(err, sql.ErrNoRows):
		return src, err
	}

	f, err := s.queryImport(ctx, `
		SELECT `+importColumns+` FROM imports
		WHERE session_id = ? AND platform = ? AND external_id = ? AND status = 'failed'
		ORDER BY id DESC LIMIT 1`, sessionID, platform, externalID)
	switch {
	case err == nil:
		src.Failed = &f
	case !errors.Is(err, ErrNotFound):
		return src, err
	}
	return src, nil
}

// execOne выполняет UPDATE одной строки; ErrNotFound, если строки нет.
func (s *Store) execOne(ctx context.Context, q string, args ...any) error {
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListImportMatches возвращает матчи, созданные загрузкой, по порядку в сессии.
func (s *Store) ListImportMatches(ctx context.Context, importID int64) ([]Match, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+matchColumns+" FROM matches WHERE import_id = ? ORDER BY ordinal", importID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Match{}
	for rows.Next() {
		m, err := scanMatch(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}
