// Package ingest принимает загруженные файлы демок: распаковывает, проверяет размер,
// считает sha256, отсекает дубликаты и ставит матч в очередь на обработку.
package ingest

import (
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/klauspost/compress/zstd"

	"cs2stats/internal/store"
)

type Status string

const (
	Accepted  Status = "accepted"
	Duplicate Status = "duplicate"
	Failed    Status = "error"
)

// FileResult — итог обработки одного загруженного файла.
type FileResult struct {
	FileName  string `json:"fileName"`
	Status    Status `json:"status"`
	MatchID   int64  `json:"matchId,omitempty"`
	SessionID int64  `json:"sessionId,omitempty"`
	Error     string `json:"error,omitempty"`
}

type format int

const (
	fmtDem format = iota
	fmtGzip
	fmtBzip2
	fmtZstd
	fmtZip
)

var suffixes = []struct {
	suffix string
	format format
}{
	{".dem.gz", fmtGzip},
	{".dem.bz2", fmtBzip2},
	{".dem.zst", fmtZstd},
	{".dem", fmtDem},
	{".zip", fmtZip},
}

// SupportedExtensions — для подсказок в интерфейсе и сообщений об ошибках.
const SupportedExtensions = ".dem, .dem.gz, .dem.bz2, .dem.zst, .zip"

func detectFormat(name string) (format, error) {
	lower := strings.ToLower(name)
	for _, s := range suffixes {
		if strings.HasSuffix(lower, s.suffix) {
			return s.format, nil
		}
	}
	return 0, fmt.Errorf("неподдерживаемый формат файла, ожидается %s", SupportedExtensions)
}

type Service struct {
	Store   *store.Store
	Demos   Storage
	TmpDir  string // должен быть на той же ФС, что и хранилище демок
	MaxSize int64  // лимит распакованного размера демки

	// mu согласует запись файла демки и создание матча с удалением файлов (RemoveDemos):
	// иначе удаление может стереть файл, который параллельная загрузка той же демки только что сохранила.
	mu sync.Mutex
}

// Ingest принимает один файл в сессию. Ошибки по файлу возвращаются в FileResult,
// чтобы остальные файлы загрузки обрабатывались независимо.
func (s *Service) Ingest(ctx context.Context, sessionID int64, name string, r io.Reader) FileResult {
	return s.ingest(ctx, sessionID, name, r, nil)
}

// IngestImport принимает демку, скачанную по ссылке на матч: матч запоминает источник
// и встаёт в сессии по времени игры (store.AddMatchFrom).
func (s *Service) IngestImport(ctx context.Context, sessionID int64, name string, r io.Reader, src *store.MatchSource) FileResult {
	return s.ingest(ctx, sessionID, name, r, src)
}

func (s *Service) ingest(ctx context.Context, sessionID int64, name string, r io.Reader, src *store.MatchSource) FileResult {
	res := FileResult{FileName: name}
	fail := func(err error) FileResult {
		res.Status, res.Error = Failed, err.Error()
		return res
	}

	tmpPath, sum, err := s.unpack(name, r)
	if err != nil {
		return fail(err)
	}
	defer os.Remove(tmpPath) // после Save файла по этому пути уже нет

	// распаковка — долгая часть — идёт вне блокировки; под ней только проверка, rename и insert
	s.mu.Lock()
	defer s.mu.Unlock()

	duplicate := func(m store.Match) FileResult {
		res.Status, res.MatchID, res.SessionID = Duplicate, m.ID, m.SessionID
		return res
	}
	if m, err := s.Store.FindMatchBySHA(ctx, sum); err == nil {
		return duplicate(m)
	} else if !errors.Is(err, store.ErrNotFound) {
		return fail(err)
	}

	if err := s.Demos.Save(sum, tmpPath); err != nil {
		return fail(err)
	}
	m, err := s.Store.AddMatchFrom(ctx, sessionID, sum, name, src)
	var dup *store.DuplicateError
	switch {
	case errors.As(err, &dup):
		// та же демка параллельно загружена другим запросом; файл идентичен, удалять нельзя
		return duplicate(dup.Existing)
	case err != nil:
		s.Demos.Delete(sum)
		return fail(err)
	}
	res.Status, res.MatchID, res.SessionID = Accepted, m.ID, m.SessionID
	return res
}

// RemoveDemos удаляет файлы демок удалённых матчей. Файл sha, для которого уже снова
// существует матч (демку загрузили заново), не трогается. Ошибки по файлам объединяются.
func (s *Service) RemoveDemos(ctx context.Context, shas []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var errs []error
	for _, sha := range shas {
		_, err := s.Store.FindMatchBySHA(ctx, sha)
		switch {
		case err == nil:
			continue
		case !errors.Is(err, store.ErrNotFound):
			errs = append(errs, fmt.Errorf("демка %s: %w", sha, err))
			continue
		}
		if err := s.Demos.Delete(sha); err != nil {
			errs = append(errs, fmt.Errorf("удаление демки %s: %w", sha, err))
		}
	}
	return errors.Join(errs...)
}

// unpack распаковывает файл во временный файл, проверяя лимит размера, и возвращает его путь и sha256.
func (s *Service) unpack(name string, r io.Reader) (path, sum string, err error) {
	f, err := detectFormat(name)
	if err != nil {
		return "", "", err
	}

	var src io.Reader
	switch f {
	case fmtDem:
		src = r
	case fmtGzip:
		gz, err := gzip.NewReader(r)
		if err != nil {
			return "", "", fmt.Errorf("архив gzip повреждён: %w", err)
		}
		defer gz.Close()
		src = gz
	case fmtBzip2:
		src = bzip2.NewReader(r)
	case fmtZstd:
		zr, err := zstd.NewReader(r)
		if err != nil {
			return "", "", fmt.Errorf("архив zstd повреждён: %w", err)
		}
		defer zr.Close()
		src = zr
	case fmtZip:
		rc, cleanup, err := s.openZip(r)
		if err != nil {
			return "", "", err
		}
		defer cleanup()
		src = rc
	}
	return s.writeTemp(src)
}

// openZip сохраняет zip во временный файл (archive/zip нужен ReaderAt) и открывает единственную демку в нём.
func (s *Service) openZip(r io.Reader) (io.Reader, func(), error) {
	tmp, err := os.CreateTemp(s.TmpDir, "upload-*.zip")
	if err != nil {
		return nil, nil, err
	}
	removeTmp := func() { tmp.Close(); os.Remove(tmp.Name()) }

	if n, err := io.Copy(tmp, io.LimitReader(r, s.MaxSize+1)); err != nil {
		removeTmp()
		return nil, nil, fmt.Errorf("чтение файла: %w", err)
	} else if n > s.MaxSize {
		removeTmp()
		return nil, nil, s.tooLarge()
	}
	info, _ := tmp.Stat()
	zr, err := zip.NewReader(tmp, info.Size())
	if err != nil {
		removeTmp()
		return nil, nil, fmt.Errorf("архив zip повреждён: %w", err)
	}
	var dems []*zip.File
	for _, zf := range zr.File {
		if !zf.FileInfo().IsDir() && strings.HasSuffix(strings.ToLower(zf.Name), ".dem") {
			dems = append(dems, zf)
		}
	}
	if len(dems) != 1 {
		removeTmp()
		return nil, nil, fmt.Errorf("архив должен содержать ровно одну демку .dem, найдено: %d", len(dems))
	}
	rc, err := dems[0].Open()
	if err != nil {
		removeTmp()
		return nil, nil, fmt.Errorf("архив zip повреждён: %w", err)
	}
	return rc, func() { rc.Close(); removeTmp() }, nil
}

func (s *Service) writeTemp(src io.Reader) (path, sum string, err error) {
	tmp, err := os.CreateTemp(s.TmpDir, "upload-*.dem")
	if err != nil {
		return "", "", err
	}
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(tmp, h), io.LimitReader(src, s.MaxSize+1))
	if err != nil {
		return "", "", fmt.Errorf("чтение или распаковка файла: %w", err)
	}
	if n > s.MaxSize {
		return "", "", s.tooLarge()
	}
	if n == 0 {
		return "", "", errors.New("файл пуст")
	}
	if err := tmp.Close(); err != nil {
		return "", "", err
	}
	return tmp.Name(), hex.EncodeToString(h.Sum(nil)), nil
}

func (s *Service) tooLarge() error {
	return fmt.Errorf("файл превышает лимит %d МБ после распаковки", s.MaxSize>>20)
}
