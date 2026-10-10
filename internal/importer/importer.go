package importer

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cs2stats/internal/ingest"
	"cs2stats/internal/store"
)

// MaxLinks — сколько ссылок можно добавить за раз: каждую проверяем у платформы.
const MaxLinks = 20

const pollInterval = 30 * time.Second

var (
	ErrEmptyInput = errors.New("вставьте ссылку на матч")
	ErrTooMany    = fmt.Errorf("за раз можно добавить не больше %d ссылок", MaxLinks)
)

// Статусы результата добавления ссылки.
const (
	AddAccepted    = "accepted"    // загрузка создана
	AddRetried     = "retried"     // упавшая загрузка этой сессии поставлена на повтор
	AddExists      = "exists"      // матч уже есть
	AddDownloading = "downloading" // матч уже скачивается
	AddError       = "error"
)

// AddResult — итог добавления одной ссылки.
type AddResult struct {
	URL          string `json:"url"`
	Status       string `json:"status"`
	Platform     string `json:"platform,omitempty"`
	ExternalID   string `json:"externalId,omitempty"`
	ImportID     int64  `json:"importId,omitempty"`
	SessionID    int64  `json:"sessionId,omitempty"`
	SessionTitle string `json:"sessionTitle,omitempty"`
	SessionDate  string `json:"sessionDate,omitempty"`
	MatchID      int64  `json:"matchId,omitempty"`
	Ordinal      int    `json:"ordinal,omitempty"`
	Error        string `json:"error,omitempty"`
}

// Importer — проверка ссылок и фоновое скачивание демок по загрузкам из store.
type Importer struct {
	Store      *store.Store
	Ingest     *ingest.Service
	Platforms  []Platform
	Client     *http.Client
	TmpDir     string // на той же ФС, что и хранилище демок
	MaxSize    int64  // лимит размера одной демки
	WakeParser func()
	Log        *slog.Logger
	// Hosts — домены, к которым разрешены запросы при скачивании (с поддоменами).
	Hosts []string
	// IdleTimeout — сколько ждать новых данных при скачивании; ProgressEvery — как часто писать прогресс.
	IdleTimeout   time.Duration
	ProgressEvery time.Duration

	wake  chan struct{}
	addMu sync.Mutex // проверка дубликатов и создание загрузок

	mu      sync.Mutex // текущая загрузка
	current struct {
		id, session int64
		cancel      context.CancelFunc
	}
}

// New собирает Importer для FastCup и Cybershoke.
func New(st *store.Store, ing *ingest.Service, tmpDir string, maxSize int64, wakeParser func(), log *slog.Logger) *Importer {
	im := &Importer{
		Store: st, Ingest: ing, Platforms: Platforms(), TmpDir: tmpDir, MaxSize: maxSize,
		WakeParser: wakeParser, Log: log, IdleTimeout: 60 * time.Second, ProgressEvery: time.Second,
	}
	for _, p := range im.Platforms {
		im.Hosts = append(im.Hosts, p.Hosts()...)
	}
	im.Client = im.NewClient()
	return im
}

// NewClient — HTTP-клиент, который ходит только на домены Hosts, в том числе при перенаправлениях.
func (im *Importer) NewClient() *http.Client {
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("слишком много перенаправлений")
			}
			if !HostAllowed(req.URL.Hostname(), im.Hosts) {
				return errForeignHost
			}
			return nil
		},
	}
}

var errForeignHost = userErr("Адрес демки ведёт на сайт вне FastCup и Cybershoke, скачивание остановлено.")
var errNoResponse = userErr("Сайт с демкой не ответил. Файл не скачан.")

func (im *Importer) wakeCh() chan struct{} {
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.wake == nil {
		im.wake = make(chan struct{}, 1)
	}
	return im.wake
}

// Wake будит фоновое скачивание после добавления загрузок.
func (im *Importer) Wake() {
	select {
	case im.wakeCh() <- struct{}{}:
	default:
	}
}

// Cancel прерывает скачивание загрузки, если оно идёт.
func (im *Importer) Cancel(importID int64) {
	im.cancelIf(func(id, _ int64) bool { return id == importID })
}

// CancelSession прерывает скачивание загрузки сессии, если оно идёт.
func (im *Importer) CancelSession(sessionID int64) {
	im.cancelIf(func(_, session int64) bool { return session == sessionID })
}

func (im *Importer) cancelIf(match func(id, session int64) bool) {
	im.mu.Lock()
	defer im.mu.Unlock()
	if im.current.cancel != nil && match(im.current.id, im.current.session) {
		im.current.cancel()
	}
}

func (im *Importer) platform(name string) Platform {
	for _, p := range im.Platforms {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

// Add проверяет ссылки из ввода и ставит новые загрузки в очередь. Результаты — в порядке ссылок,
// загрузки создаются по времени игры. Ошибка возвращается только для ввода целиком.
func (im *Importer) Add(ctx context.Context, sessionID int64, text string) ([]AddResult, error) {
	urls := SplitURLs(text)
	if len(urls) == 0 {
		return nil, ErrEmptyInput
	}
	if len(urls) > MaxLinks {
		return nil, ErrTooMany
	}
	im.addMu.Lock()
	defer im.addMu.Unlock()

	results := make([]AddResult, len(urls))
	type pending struct {
		i     int
		link  Link
		match Match
	}
	var accepted []pending
	firstOf := map[string]int{} // повтор того же матча в одном вводе получает результат первой ссылки
	for i, raw := range urls {
		r := &results[i]
		r.URL = raw
		link, err := ParseURL(im.Platforms, raw)
		if err != nil {
			r.Status, r.Error = AddError, UserMessage(err, "Не удалось разобрать ссылку.")
			continue
		}
		r.URL, r.Platform, r.ExternalID = link.URL, link.Platform.Name(), link.ID
		key := link.Platform.Name() + ":" + link.ID
		if _, ok := firstOf[key]; ok {
			continue // итог скопируется из первой ссылки ниже
		}
		firstOf[key] = i
		if err := im.checkLink(ctx, sessionID, link, r); err != nil {
			return nil, err
		}
		if r.Status != "" {
			continue
		}
		m, err := link.Platform.Resolve(ctx, im.Client, link.ID)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			r.Status, r.Error = AddError, UserMessage(err, "Не удалось получить данные матча с "+link.Platform.Title()+".")
			continue
		}
		accepted = append(accepted, pending{i, link, m})
	}

	// очередь — по времени игры: раньше скачается более ранний матч
	sort.SliceStable(accepted, func(a, b int) bool {
		ma, mb := accepted[a].match, accepted[b].match
		if !ma.StartedAt.Equal(mb.StartedAt) {
			return ma.StartedAt.Before(mb.StartedAt)
		}
		return ma.Number < mb.Number
	})
	for _, p := range accepted {
		x, err := im.Store.AddImport(ctx, sessionID, p.link.URL, p.link.Platform.Name(), p.link.ID)
		if err != nil {
			return nil, err
		}
		results[p.i].Status, results[p.i].ImportID, results[p.i].SessionID = AddAccepted, x.ID, sessionID
	}
	for i := range results { // повторы в вводе получают итог первой ссылки
		if j := firstOf[results[i].Platform+":"+results[i].ExternalID]; j != i && results[i].Status != AddError {
			url := results[i].URL
			results[i] = results[j]
			results[i].URL = url
		}
	}
	return results, nil
}

// checkLink заполняет r, если матч уже есть, скачивается или его упавшую загрузку можно повторить.
func (im *Importer) checkLink(ctx context.Context, sessionID int64, link Link, r *AddResult) error {
	number, _ := strconv.ParseInt(link.ID, 10, 64)
	src, err := im.Store.FindImportSource(ctx, sessionID, link.Platform.Name(), link.ID, number)
	if err != nil {
		return err
	}
	switch {
	case src.Match != nil:
		r.Status, r.MatchID, r.Ordinal = AddExists, src.Match.ID, src.Match.Ordinal
		return im.fillSession(ctx, r, src.Match.SessionID)
	case src.Active != nil:
		r.Status, r.ImportID = AddDownloading, src.Active.ID
		return im.fillSession(ctx, r, src.Active.SessionID)
	case src.Failed != nil:
		x, err := im.Store.RetryImport(ctx, src.Failed.ID)
		if err != nil {
			return err
		}
		r.Status, r.ImportID, r.SessionID = AddRetried, x.ID, sessionID
	}
	return nil
}

func (im *Importer) fillSession(ctx context.Context, r *AddResult, sessionID int64) error {
	s, err := im.Store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	r.SessionID, r.SessionTitle, r.SessionDate = s.ID, s.Title, s.Date
	return nil
}

// Run скачивает загрузки из очереди по одной, пока ctx не отменён.
func (im *Importer) Run(ctx context.Context) error {
	if n, err := im.Store.ResetDownloading(ctx); err != nil {
		return err
	} else if n > 0 {
		im.Log.Info("прерванные скачивания возвращены в очередь", "count", n)
	}
	wake := im.wakeCh()
	for {
		im.drain(ctx)
		select {
		case <-ctx.Done():
			return nil
		case <-wake:
		case <-time.After(pollInterval):
		}
	}
}

func (im *Importer) drain(ctx context.Context) {
	for ctx.Err() == nil {
		x, ok, err := im.Store.ClaimNextImport(ctx)
		if err != nil {
			if ctx.Err() == nil {
				im.Log.Error("очередь скачивания", "err", err)
			}
			return
		}
		if !ok {
			return
		}
		im.process(ctx, x)
	}
}

// process скачивает и принимает демки одной загрузки.
func (im *Importer) process(parent context.Context, x store.Import) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	im.mu.Lock()
	im.current.id, im.current.session, im.current.cancel = x.ID, x.SessionID, cancel
	im.mu.Unlock()
	defer func() {
		im.mu.Lock()
		im.current.id, im.current.session, im.current.cancel = 0, 0, nil
		im.mu.Unlock()
	}()
	// загрузку могли удалить между claim и регистрацией отмены
	if _, err := im.Store.GetImport(ctx, x.ID); err != nil {
		return
	}

	log := im.Log.With("import", x.ID, "platform", x.Platform, "match", x.ExternalID)
	results, errMsg := im.run(ctx, x)
	if parent.Err() != nil {
		return // остановка сервиса: загрузка останется downloading и вернётся в очередь при старте
	}
	if ctx.Err() != nil {
		log.Info("скачивание отменено")
		return // загрузку или сессию удалили
	}
	if results == nil {
		results = []ingest.FileResult{} // в JSON — [], а не null: интерфейс перебирает итог по картам
	}
	data, _ := json.Marshal(results)
	if err := im.Store.FinishImport(context.WithoutCancel(ctx), x.ID, errMsg, data); err != nil && !errors.Is(err, store.ErrNotFound) {
		log.Error("сохранение итога загрузки", "err", err)
	}
	if errMsg != "" {
		log.Warn("загрузка завершилась с ошибкой", "err", errMsg)
	}
	for _, r := range results {
		if r.Status == ingest.Accepted {
			im.WakeParser()
			break
		}
	}
}

// run возвращает итог по картам и текст первой ошибки.
func (im *Importer) run(ctx context.Context, x store.Import) ([]ingest.FileResult, string) {
	p := im.platform(x.Platform)
	if p == nil {
		return nil, "Платформа " + x.Platform + " не поддерживается."
	}
	m, err := p.Resolve(ctx, im.Client, x.ExternalID)
	if err != nil {
		return nil, UserMessage(err, "Не удалось получить данные матча с "+p.Title()+".")
	}
	var results []ingest.FileResult
	errMsg := ""
	fail := func(name, msg string) {
		results = append(results, ingest.FileResult{FileName: name, Status: ingest.Failed, Error: msg})
		if errMsg == "" {
			errMsg = msg
		}
	}
	for _, d := range m.Demos {
		file, name, err := im.download(ctx, x.ID, d)
		if err != nil {
			if ctx.Err() != nil {
				return results, ""
			}
			fail(d.Name, UserMessage(err, errNoResponse.Error()))
			continue
		}
		for _, r := range im.accept(ctx, x, m, d, file, name) {
			results = append(results, r)
			if r.Status == ingest.Failed && errMsg == "" {
				errMsg = r.Error
			}
		}
		os.Remove(file)
	}
	return results, errMsg
}

// idleReader прерывает скачивание, если данные не приходят дольше timeout.
type idleReader struct {
	r     io.Reader
	timer *time.Timer
	d     time.Duration
}

func (ir *idleReader) Read(p []byte) (int, error) {
	n, err := ir.r.Read(p)
	ir.timer.Reset(ir.d)
	return n, err
}

// download скачивает файл демки во временный файл и возвращает его путь и имя для приёма.
func (im *Importer) download(parent context.Context, importID int64, d Demo) (string, string, error) {
	u, err := url.Parse(d.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !HostAllowed(u.Hostname(), im.Hosts) {
		return "", "", errForeignHost
	}
	// архив серии содержит несколько демок, лимит — на каждую
	limit := im.MaxSize * int64(max(1, len(d.Maps)))

	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.URL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := im.Client.Do(req)
	if err != nil {
		if parent.Err() != nil {
			return "", "", parent.Err()
		}
		var ue *UserError
		if errors.As(err, &ue) {
			return "", "", ue
		}
		return "", "", errNoResponse
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", userErr(fmt.Sprintf("Сайт с демкой ответил ошибкой %d. Файл не скачан.", resp.StatusCode))
	}
	var total *int64
	if resp.ContentLength > 0 {
		if resp.ContentLength > limit {
			return "", "", sizeErr(limit)
		}
		total = &resp.ContentLength
	}

	f, err := os.CreateTemp(im.TmpDir, "import-*.part")
	if err != nil {
		return "", "", err
	}
	ok := false
	defer func() {
		if !ok {
			f.Close()
			os.Remove(f.Name())
		}
	}()

	timer := time.AfterFunc(im.IdleTimeout, cancel)
	defer timer.Stop()
	body := io.LimitReader(&idleReader{r: resp.Body, timer: timer, d: im.IdleTimeout}, limit+1)

	progress := context.WithoutCancel(parent)
	im.Store.UpdateImportProgress(progress, importID, 0, total)
	var done int64
	last := time.Now()
	buf := make([]byte, 256<<10)
	for {
		n, rerr := body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return "", "", err
			}
			done += int64(n)
			if done > limit {
				return "", "", sizeErr(limit)
			}
			if time.Since(last) >= im.ProgressEvery {
				im.Store.UpdateImportProgress(progress, importID, done, total)
				last = time.Now()
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if parent.Err() != nil {
				return "", "", parent.Err()
			}
			return "", "", errNoResponse
		}
	}
	if total != nil && done != *total {
		return "", "", errNoResponse // соединение оборвалось раньше конца файла
	}
	im.Store.UpdateImportProgress(progress, importID, done, total)
	if err := f.Close(); err != nil {
		return "", "", err
	}
	ok = true
	return f.Name(), fileName(resp, d), nil
}

func sizeErr(limit int64) error {
	return userErr(fmt.Sprintf("Файл демки больше лимита %d МБ. Файл не скачан.", limit>>20))
}

// fileName — имя из Content-Disposition, иначе из пути адреса, иначе имя от платформы.
func fileName(resp *http.Response, d Demo) string {
	if _, params, err := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); err == nil {
		if name := path.Base(strings.ReplaceAll(params["filename"], `\`, "/")); name != "." && name != "/" && name != "" {
			return name
		}
	}
	if name := path.Base(resp.Request.URL.Path); path.Ext(name) != "" {
		return name
	}
	return d.Name
}

var mapSuffix = regexp.MustCompile(`(?i)_map(\d+)\.dem$`)

// accept принимает скачанный файл: .dem/.dem.* целиком, zip — каждую демку архива серии по порядку карт.
func (im *Importer) accept(ctx context.Context, x store.Import, m Match, d Demo, file, name string) []ingest.FileResult {
	source := func(mi MapInfo) *store.MatchSource {
		id := x.ID
		return &store.MatchSource{ImportID: &id, Platform: x.Platform, Number: m.Number, Map: mi.Number, PlayedAt: mi.StartedAt}
	}
	mapAt := func(number, index int) MapInfo {
		for _, mi := range d.Maps {
			if mi.Number == number {
				return mi
			}
		}
		if index < len(d.Maps) {
			return d.Maps[index]
		}
		if len(d.Maps) > 0 {
			return d.Maps[len(d.Maps)-1]
		}
		return MapInfo{Number: max(number, 1), StartedAt: m.StartedAt}
	}

	if !strings.HasSuffix(strings.ToLower(name), ".zip") {
		f, err := os.Open(file)
		if err != nil {
			return []ingest.FileResult{{FileName: name, Status: ingest.Failed, Error: "Скачанный файл не найден."}}
		}
		defer f.Close()
		return []ingest.FileResult{im.Ingest.IngestImport(ctx, x.SessionID, name, f, source(mapAt(1, 0)))}
	}

	zr, err := zip.OpenReader(file)
	if err != nil {
		return []ingest.FileResult{{FileName: name, Status: ingest.Failed, Error: "Архив с демкой повреждён."}}
	}
	defer zr.Close()
	type entry struct {
		f   *zip.File
		num int // номер карты из суффикса _mapN, 0 — без суффикса
	}
	var dems []entry
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || !strings.HasSuffix(strings.ToLower(f.Name), ".dem") {
			continue
		}
		e := entry{f: f}
		if sm := mapSuffix.FindStringSubmatch(f.Name); sm != nil {
			e.num, _ = strconv.Atoi(sm[1])
		}
		dems = append(dems, e)
	}
	if len(dems) == 0 {
		return []ingest.FileResult{{FileName: name, Status: ingest.Failed, Error: "В архиве нет демок."}}
	}
	sort.SliceStable(dems, func(i, j int) bool { return dems[i].num < dems[j].num })

	var results []ingest.FileResult
	for i, e := range dems {
		entryName := path.Base(e.f.Name)
		if ctx.Err() != nil {
			break
		}
		rc, err := e.f.Open()
		if err != nil {
			results = append(results, ingest.FileResult{FileName: entryName, Status: ingest.Failed, Error: "Архив с демкой повреждён."})
			continue
		}
		num := e.num
		if num == 0 {
			num = i + 1
		}
		results = append(results, im.Ingest.IngestImport(ctx, x.SessionID, entryName, rc, source(mapAt(num, i))))
		rc.Close()
	}
	return results
}
