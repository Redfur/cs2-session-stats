# Design

## Context

Приём файла уже устроен как `ingest.Service.Ingest(ctx, sessionID, name, io.Reader)`. Внутри формат определяется по суффиксу имени (`.dem`, `.dem.gz`, `.dem.bz2`, `.dem.zst`, `.zip` ровно с одной демкой), дальше лимит `MaxDemoSize`, sha256, дедуп и `store.AddMatch`. Номер матча (`ordinal`) назначается в момент вставки, в конец сессии. В `matches.sha256` стоит `NOT NULL UNIQUE`, а строку матча без демки создать нельзя. Воркер разбора (`internal/worker`) один, последовательный, просыпается по `Wake()` и восстанавливает очередь при старте (`ResetParsing`, `RequeueOutdated`). Страница сессии опрашивает `GET /api/sessions/{id}` раз в 3 секунды, пока есть матчи в работе (`isProcessing`).

Данные платформ проверены на живых матчах 2026-10-10:

- **FastCup.**
  - Страница `cs2.fastcup.net/matches/{id}` берёт данные из `POST https://hasura.fastcup.net/v1/graphql`, запрос `__GetMatch` с переменными `{matchId, gameId: 3}`. Анонимно, но Hasura пропускает **только запросы из allowlist**: на свой запрос ответ `query is not allowed`, дословная копия проходит.
  - Нужные поля: `match.status` (`FINISHED`), `bestOf`, `replayExpirationDate` (около 30 дней после матча), `maps(order_by:{number:asc}) { number, replays { url, createdAt } }`.
  - Демка лежит по адресу `https://replays.fastcup.net/<n>/<match>_<map>_<...>-<mapname>.dem`. Это несжатый `.dem` около 300 МБ, без Content-Length.
- **Cybershoke.**
  - `POST https://cybershoke.net/api/api/v1/custom-matches/lobbys/info` с телом `{"id_lobby": N}`, анонимно.
  - Нужные поля: `demo.status` (4 = готова) и `demo.url_download` (`https://cdn-de-1.cybershoke.net/demos/{id}?series=1`), плюс `match_settings.bo` и `match_stats.bo.{n}`.
  - Файл — zip, есть Content-Length и `Content-Disposition: attachment; filename="match_{id}.zip"`, Range не поддерживается.
  - Bo1 содержит `match_{id}.dem`. Bo3, сыгранная 2:0 (лобби 12578063, 392 МБ), содержит `match_{id}_map1.dem` и `match_{id}_map2.dem`: один архив на всю серию.
  - Страницы бывают с языковым префиксом (`/ru/match/…`, `/de/match/…`).

## Goals / Non-Goals

**Goals:**
- скачивание выполняется на сервере в фоне;
- приём файла общий с ручной загрузкой;
- платформы легко добавлять (следующая — Faceit);
- после перезапуска сервиса загрузки продолжаются.

**Non-Goals:**
- параллельное скачивание;
- докачка после обрыва (Range у Cybershoke нет);
- автоповтор по расписанию;
- дата матча и любые другие данные со страницы платформы, кроме адресов демок.

## Decisions

### 1. Отдельная очередь `imports`, схема и статусы `matches` не меняются

Миграция `004_imports.sql`:

```sql
CREATE TABLE imports (
    id          INTEGER PRIMARY KEY,
    session_id  INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    url         TEXT    NOT NULL,          -- нормализованная ссылка на страницу матча
    platform    TEXT    NOT NULL,          -- fastcup | cybershoke
    external_id TEXT    NOT NULL,          -- номер матча на платформе
    status      TEXT    NOT NULL DEFAULT 'queued', -- queued | downloading | done | failed
    error       TEXT    NOT NULL DEFAULT '',
    bytes_done  INTEGER NOT NULL DEFAULT 0,
    bytes_total INTEGER,                   -- NULL, если размер неизвестен
    results     TEXT    NOT NULL DEFAULT '[]', -- JSON []ingest.FileResult по картам
    created_at  TEXT    NOT NULL,
    finished_at TEXT
);
CREATE INDEX imports_source ON imports(platform, external_id);
CREATE INDEX imports_status ON imports(status);
ALTER TABLE matches ADD COLUMN import_id INTEGER REFERENCES imports(id) ON DELETE SET NULL;
```

Матч создаётся только после скачивания, через тот же `Ingest`. Альтернатива отвергнута: создавать строку матча со статусом `downloading` сразу при добавлении ссылки. Тогда `sha256` пришлось бы сделать nullable, а SQLite для этого пересобирает `matches` вместе с внешними ключами `match_players` и `match_duels`. Ещё пришлось бы править `requeueSet`, удаление демок, дедуп и все места, где подразумевается, что у матча есть демка.

Цена решения в том, что номер матча назначается при приёме демки. Если файл загрузить вручную, пока идёт скачивание, он окажется раньше. Порядок можно поправить существующей перестановкой матчей. Пользователь выбрал этот вариант осознанно.

`matches.import_id` хранит происхождение матча и нужен, чтобы проверить «уже загружен» до скачивания (решение 6). Колонка добавляется через `ADD COLUMN` без пересборки, у существующих строк остаётся NULL.

### 2. Ссылка разбирается сразу, демки ищутся в фоне

`POST /api/sessions/{id}/imports` только распознаёт платформу и номер матча (решение 3) и проверяет дубликат. Обращение к API платформы делается в фоне, в начале скачивания. Тогда у первой попытки и у «Повторить» один путь выполнения. Ошибка «демка ещё не готова» исправляется повтором. Ответ на добавление не зависит от доступности платформы.

Альтернатива: искать демки синхронно, чтобы ошибку было видно сразу в поле. Отвергнута: два места обработки одних и тех же ошибок. При этом ошибка всё равно появляется в списке загрузок в течение нескольких секунд.

### 3. Пакет `internal/importer` и интерфейс платформы

```go
type Demo struct {
    URL  string // адрес файла демки или архива серии
    Name string // имя для Ingest, если его не дал ответ сервера
}

type Platform interface {
    Name() string                                   // "fastcup", "cybershoke"
    Hosts() []string                                // разрешённые домены (с поддоменами)
    Parse(u *url.URL) (externalID string, ok bool)
    Resolve(ctx context.Context, c *http.Client, externalID string) ([]Demo, error)
}
```

- **`ParseURL(raw)`** обрезает пробелы, принимает схемы `http`/`https`, убирает `www.`, затем перебирает платформы.
  - **FastCup:** хост `cs2.fastcup.net` (и `fastcup.net`), путь `/matches/{digits}` с любым хвостом.
  - **Cybershoke:** хост `cybershoke.net`, путь `[/{lang}]/match/{digits}`. `{lang}` — 2–5 букв или дефис.
  - Ошибка разбора одна для всех случаев: «поддерживаются ссылки на матчи FastCup (https://cs2.fastcup.net/matches/12345) и Cybershoke (https://cybershoke.net/match/12345)».
- **FastCup.** Дословный текст `__GetMatch` лежит в `internal/importer/fastcup_get_match.graphql` (`go:embed`), снятый со страницы. Allowlist сверяет текст, поэтому файл не форматируется и не правится.
  - Карты сортируются по `number`, у карты берётся последний replay по `createdAt`, карты без replay пропускаются.
  - Ошибки:
    - `match == null` → «матч не найден»;
    - `status != FINISHED` → «матч ещё не закончен»;
    - нет replay и истёк `replayExpirationDate` → «FastCup удалил демку (хранится около 30 дней)»;
    - нет replay, срок не истёк → «демка ещё не готова»;
    - `errors[]` от Hasura (в том числе `query is not allowed`) → «FastCup изменил API, получить демку не удалось».

    Ко всем ошибкам добавляется «загрузите файл вручную».
- **Cybershoke.** `lobbys/info`.
  - Ошибки:
    - `result != success` или пустой `data` → «матч не найден»;
    - `demo.status != 4` или пустой `url_download` → «демка ещё не готова».
  - Возвращается один `Demo`. Серия раскладывается по картам при приёме архива (решение 5).
- Платформы собраны в реестр, порядок фиксированный. Faceit добавится отдельной реализацией интерфейса.

### 4. Фоновое скачивание `importer.Importer`

Это отдельная горутина, а не часть воркера разбора: скачивание упирается в сеть, разбор — в CPU, и сотни мегабайт загрузки не должны задерживать разбор уже принятых матчей. Устройство повторяет `worker.Worker`:
- `New(store, ingest, platforms, client, tmpDir, maxSize, wakeParser, log)`, `Run(ctx)`, `Wake()`;
- при старте `ResetDownloading` (`downloading` → `queued`, `bytes_done = 0`);
- затем цикл: `drain` → ожидание `Wake` / `ctx.Done` / таймера 30 с;
- `ClaimNextImport` атомарно переводит самую раннюю `queued` в `downloading` (`UPDATE … WHERE id = (SELECT … ORDER BY id LIMIT 1) RETURNING`).

Загрузки обрабатываются строго по одной, в порядке `id`. Так соблюдается требование о порядке матчей.

**Отмена.** Каждая загрузка получает свой `context.WithCancel`. Importer хранит текущую пару `(importID, sessionID, cancel)` под мьютексом. `Cancel(importID)` и `CancelSession(sessionID)` вызываются из API после удаления загрузки или сессии. Если загрузку отменили, ошибка не записывается, временные файлы удаляются. Остановка сервиса оставляет `downloading` в БД, при старте загрузка вернётся в очередь.

**Прогресс.** Тело ответа читается через счётчик. `UpdateImportProgress(id, done, total)` вызывается не чаще раза в секунду и в конце. `bytes_total` берётся из Content-Length, если он есть. У серии FastCup из нескольких файлов прогресс считается по текущему файлу. Номер карты в строке не показывается: этого хватает, чтобы видеть движение.

### 5. Скачивание и приём файла

1. **Проверка адреса.** Адрес демки проверяется по `Hosts()` всех платформ. Чужой хост → ошибка «адрес демки вне разрешённых сайтов».
2. **HTTP-клиент**:
   - `CheckRedirect` проверяет каждый переход тем же списком хостов и ограничивает число переходов (≤5);
   - `Transport` с таймаутами соединения, TLS и ожидания заголовков (по 30 с);
   - тело читается через reader с таймаутом простоя (60 с без данных → ошибка);
   - общий таймаут на файл не ставится: 400 МБ на медленном канале — нормальная ситуация.
3. **Временный файл.** Тело пишется в `DataDir/tmp/import-*.part` с ограничением `MaxDemoSize + 1` байт сжатого потока. Превышение → «файл больше лимита …», файл удаляется.
4. **Имя.** Берётся из `Content-Disposition`, если его нет — из последнего сегмента пути URL, если и его нет — `Demo.Name`. Имя нужно `Ingest`, чтобы определить формат.
5. **Приём:**
   - **zip:** файл открывается `archive/zip`, берутся все записи `*.dem`. Записи с суффиксом `_map{N}` сортируются по N, остальные остаются в порядке архива. Каждая запись передаётся в `Ingest(ctx, sessionID, entry.Name, rc)` отдельно. Многодемочный zip принимается только здесь: ручная загрузка по-прежнему требует ровно одну демку (`openZip` не меняется). zip без `.dem` → ошибка «в архиве нет демок».
   - **иначе:** файл передаётся в `Ingest(ctx, sessionID, name, f)` целиком, распаковка gz/bz2/zst общая.
6. **`importID` в `Ingest`.** Добавляется вариантом метода `IngestImport(ctx, sessionID, importID, name, r)`. Общая часть выносится во внутреннюю функцию. `store.AddMatch` получает `importID *int64`. Ручная загрузка передаёт `nil`, её поведение не меняется.
7. **Итог:**
   - результаты карт (`accepted` / `duplicate` / `error` с `matchId`, `sessionId`) накапливаются и пишутся в `imports.results`;
   - если есть хоть один `error` или ошибка скачивания, загрузка получает `failed` с первым текстом ошибки, иначе `done`;
   - если принят хотя бы один матч, вызывается `wakeParser()`.

Если загрузку или сессию удалили во время приёма, `FinishImport` или `FailImport` вернёт `ErrNotFound`, и это не считается ошибкой. Матч в удалённой сессии не создастся: `AddMatch` упрётся во внешний ключ, а результат игнорируется.

### 6. Дубликаты до скачивания

В `POST …/imports` одной транзакцией проверяется:
1. есть ли загрузка `(platform, external_id)` в статусе `queued`/`downloading` → 409 «уже скачивается» с её `sessionId`;
2. есть ли матч с `import_id` загрузки того же `(platform, external_id)` → 409 «уже загружен» с `sessionId` и `matchId` первого такого матча;
3. есть ли в **этой** сессии загрузка того же источника со статусом `failed` → она ставится на повтор (`RetryImport`), ответ 200 с этой загрузкой;
4. иначе создаётся новая загрузка → 201.

Матч, загруженный вручную файлом, по источнику не распознаётся. Он отловится дедупом по sha256 при приёме (результат карты `duplicate`), ценой лишнего скачивания.

### 7. API

- `POST /api/sessions/{id}/imports`, тело `{"url": "..."}`. Ответы:
  - `201` или `200` с `Import`;
  - `400 {"error"}` — неподдерживаемая ссылка или пустое тело;
  - `404` — нет сессии;
  - `409 {"error", "sessionId", "matchId"?}`.

  После создания вызывается `Importer.Wake()`.
- `POST /api/imports/{id}/retry`: `202` с `Import`; `409`, если статус не `failed`; `404`.
- `DELETE /api/imports/{id}`: удаляет запись и вызывает `Importer.Cancel(id)`; `204` или `404`. Матчи остаются, их `import_id` становится NULL.
- `DELETE /api/sessions/{id}` дополнительно вызывает `Importer.CancelSession(id)`. Записи удаляются каскадом.
- `GET /api/sessions/{id}` получает поле `imports`: загрузки сессии в статусах `queued`, `downloading`, `failed` и `done`, если в `results` есть не-`accepted`. Порядок — по `id`.
- `Import` в JSON: `{id, sessionId, url, platform, externalId, status, error?, bytesDone, bytesTotal?, results: [{fileName, status, matchId?, sessionId?, error?}], createdAt, finishedAt?}`.

### 8. Интерфейс

- **`api.ts`:**
  - тип `Import` и `ImportStatus`;
  - `createImport(sessionId, url)` — ошибка 409 возвращает `sessionId` и `matchId`, чтобы показать ссылку;
  - `retryImport(id)`, `deleteImport(id)`;
  - `SessionDetails.imports`.
- **`components/MatchLinkForm.tsx`.**
  - Строка `Input` (`type="url"`, плейсхолдер `https://cs2.fastcup.net/matches/… или https://cybershoke.net/match/…`) с `Button` «Добавить» внутри `<form>`, по образцу `InlineEdit`.
  - Ошибка показывается под полем через `Field`/`role="alert"`, для 409 — со ссылкой «Открыть сессию».
  - Форма стоит под `DropZone` (`large`) в пустой сессии и под `DropZone` (`strip`) в `CardFooter`.
- **`components/ImportList.tsx`.**
  - Список в том же стиле, что `UploadList`; общий `Bar` выносится из `UploadList.tsx` в `components/ui/ProgressBar.tsx`.
  - Строка: метка платформы (FastCup/Cybershoke), `#номер` моноширинным шрифтом со ссылкой на страницу матча (`target="_blank"`, `rel="noreferrer"`), бейдж статуса:
    - `neutral` + Clock «В очереди»;
    - `progress` + Download «Скачивается»;
    - `error` + TriangleAlert «Ошибка»;
    - `ok` + Check «Готово».
  - При скачивании: прогресс «N из M МБ · p%» или «N МБ».
  - Результаты карт: «Карта 1 → матч #3», «уже загружена → сессия», ошибка.
  - Действия: «Отменить» (`queued`/`downloading`), «Повторить» и «Убрать» (`failed`), «Убрать» (`done`).
- **`SessionPage`.**
  - Опрос включается и при `imports.some(i => i.status === 'queued' || i.status === 'downloading')`.
  - Если загрузка завершилась, а матчи ещё в очереди, опрос продолжается по `isProcessing` матчей.
- **Стили.** Только компоненты `components/ui` и токены `@theme`. Макета в Claude Design для этого блока нет, он собирается из существующих компонентов по образцу `UploadList` и `DropZone`.

## Risks / Trade-offs

- [FastCup меняет текст `__GetMatch` при обновлении фронта, и Hasura перестаёт пропускать нашу копию] → Ошибка «FastCup изменил API» с советом загрузить файл вручную. Обновление сводится к замене одного `.graphql`-файла копией со страницы (способ записан в CLAUDE.md). Тест резолвера проверяет разбор ответа, а не совпадение с живым allowlist.
- [Cybershoke или Cloudflare блокирует запросы сервера (с VPS или по User-Agent)] → Ставится обычный User-Agent браузера, ошибка показывается понятным текстом, ручная загрузка остаётся. Обходить защиту не будем.
- [Демки FastCup живут около 30 дней] → Понятная ошибка про срок хранения. В README — совет импортировать матчи в тот же вечер.
- [Неизвестный формат Bo3 FastCup: живого примера нет] → Структура `maps[]/replays[]` видна в запросе. Резолвер проверяется на синтетическом ответе из трёх карт, живая проверка — открытая задача до появления серии.
- [Порядок матчей при одновременной ручной загрузке] → Поведение описано в спеке. Есть перестановка матчей.
- [Диск: временный файл (до 400 МБ) плюс распакованные демки серии (до 1 ГБ каждая) в `DataDir/tmp`] → Скачивания идут по одному, временные файлы удаляются сразу после приёма и при отмене. Лимит сжатого потока равен `MaxDemoSize`.
- [Сервис без авторизации скачивает по присланным ссылкам] → Принимаются только распознанные страницы матчей. Все исходящие запросы — к API платформ, демкам и редиректам — проверяются по списку доменов.

## Migration Plan

1. Миграция `004` создаёт `imports` и колонку `matches.import_id` (NULL для существующих матчей). Данные не переносятся.
2. `worker.ProcessingVersion` не меняется: парсер и статистика прежние.
3. Откат: старый бинарь запускается на базе с `user_version = 4`, потому что `migrate` пропускает известные номера. Таблицу `imports` и колонку `import_id` он не видит, так как колонки матчей выбираются явно. Незавершённые загрузки при откате просто не скачиваются.

## Open Questions

- Нужна ли загрузка вручную zip-архива серии Cybershoke (несколько демок) — отдельный change, если понадобится.
- Нужно ли брать дату матча из данных платформы для подсказки даты сессии — отдельный change.
