# CLAUDE.md

Анализатор демок CS2 для дружеских 5х5. Статистика считается только по играм компании внутри **игровых сессий** (вечеров), а не по всем матчам игроков.
Матчи играются кастомными на **FastCup** и **Cybershoke**. Публичного API для скачивания демок у них нет, поэтому демки загружаются вручную.
Пользовательская документация — в `README.md`, требования — в `openspec/specs/`.

## Процесс работы

- **Любая доработка идёт через OpenSpec:** `/opsx:propose` → ревью артефактов пользователем → `/opsx:apply` → `/opsx:archive`.
  Не начинать реализацию без явной команды apply. Архивировать с синхронизацией спек в `openspec/specs/`.
- **Маленькими шагами.** Сначала минимальный change, улучшения — отдельными changes. Если в задаче всплывает работа сверх спеки, остановиться и спросить, а не расширять объём молча.
- **Фронтенд — по дизайн-системе.** Новые экраны собирать из `web/src/components/ui/` и токенов `@theme`, без разовых цветов и размеров.
  Источник дизайна — артефакт Claude Design https://claude.ai/artifact/RyiK38Ld5u2q9uQMxYhe6h (снимок макетов — `data/design-input/design/`, вне git).
- Если при реализации решение отличается от `design.md`/`tasks.md`, сразу поправить артефакты, чтобы они соответствовали коду.
- Задачу в `tasks.md` отмечать `[x]` только после выполнения её проверки. Если проверку сделать нельзя (например, нет демки нужной платформы), задача остаётся открытой, и об этом надо сказать.
- Артефакты OpenSpec, комментарии в коде, тексты UI, ошибки API и сообщения коммитов пишутся **на русском**. Ключевые слова OpenSpec (SHALL/MUST, заголовки) остаются на английском.
- **Коммиты делать самостоятельно**, без отдельного запроса, в ветку `main`. Делить по смыслу: крупную фичу не выкатывать одним коммитом и не дробить на кучу мелких.
  Обычная нарезка change: логические части реализации (например, бэкенд, фронтенд, найденные при проверке исправления) и отдельный коммит архивации OpenSpec.
  Каждый коммит должен собираться и проходить тесты.

## Команды

Go установлен в `~/.local/go` и **не лежит в PATH** shell-сессии: перед `go` выполнять `export PATH=$HOME/.local/go/bin:$PATH`. `Makefile` добавляет его в PATH сам.

```sh
make test     # go vet + go test ./... + tsc --noEmit (фронт)
make dev      # Go API на :8080 + Vite на :5173 (проксирует /api)
make build    # бинарь ./cs2stats со встроенным фронтом (-tags embedweb)
make web      # сборка фронта в internal/webui/dist
cd web && npm run lint                                      # oxlint
go run ./cmd/cs2stats parse path/to/match.dem               # разбор демки в JSON без сервиса
go run ./cmd/cs2stats reparse --all|--session ID|--match ID # поставить матчи на пересчёт
docker compose up -d --build                                # сервис на :8080, данные в ./data
docker compose exec cs2stats /cs2stats reparse --all        # CLI внутри контейнера
openspec validate <change> --strict
```

Проверка парсера на реальной демке (тест пропускается без переменной):
```sh
CS2STATS_TEST_DEMO=path.dem CS2STATS_TEST_ROUNDS=24 CS2STATS_TEST_SCORE=11:13 go test -run TestParseRealDemo -v ./internal/parser/
```

## Архитектура

Один Go-бинарь: HTTP API + фоновый воркер + раздача фронта. Хранилище — SQLite (`modernc.org/sqlite`, без CGO, WAL, `_txlock=immediate`). Демки лежат на диске.

```
cmd/cs2stats       main: serve (по умолчанию), parse FILE, reparse
internal/config    env: DATA_DIR (./data), ADDR (:8080), MAX_DEMO_SIZE (1 ГБ), STATIC_DIR
internal/parser    демка → доменная модель Match/Round/Kill/Damage (demoinfocs-golang/v5); без зависимостей от demoinfocs снаружи
internal/stats     Compute(parser.Match) → []PlayerStats; Counters и производные показатели
internal/ingest    приём файла: формат, распаковка gz/bz2/zst/zip, лимит, sha256, дедуп; Storage (LocalStorage)
internal/worker    очередь матчей; ProcessingVersion; пересчёт устаревших при старте
internal/store     SQLite: sessions, matches, match_players; миграции migrations/NNN_*.sql через PRAGMA user_version
internal/api       JSON API на net/http ServeMux; PlayerView для вывода
internal/webui     раздача SPA: embed при -tags embedweb, иначе с диска (STATIC_DIR)
web/               Vite + React + TS + react-router + Tailwind v4 + Headless UI + lucide-react; страницы /, /sessions/:id, /matches/:id, /players, /players/:id
  src/index.css      токены дизайна (@theme), шрифты Manrope и JetBrains Mono самохостом (@fontsource-variable)
  src/components/ui  компоненты дизайн-системы: каркас, кнопки, поля, бейджи, StatTable, диалоги, загрузка
  src/metrics.ts     шкала «плохо / средне / хорошо» для rating, K/D, ADR, KAST
  src/sort.ts        сортировка таблиц в адресе (useSort); src/upload.ts — очередь загрузки демок
```

Поток данных: `POST /api/sessions/{id}/demos` → `ingest` сохраняет `data/demos/<sha256>.dem` и создаёт матч `pending` → `worker.Wake()` → `parser.Parse` → `stats.Compute` → `store.SaveMatchResult`.

## Ключевые решения и инварианты

- **Сессию создаёт пользователь** (дата + название). В демках CS2 нет даты матча, поэтому группировать по времени нельзя. Порядок матчей — порядок загрузки файлов (`ordinal`).
- **В БД хранятся только сырые счётчики** (`stats.Counters`). ADR, KAST%, HS%, K/D и rating считаются при чтении. Агрегаты сессии — суммы счётчиков, поэтому они автоматически взвешены по раундам. Rating — HLTV 1.0.
- **`worker.ProcessingVersion`.** Поднимать при любом изменении `internal/parser`/`internal/stats`, которое меняет сохраняемые счётчики: при старте сервис сам пересчитает старые матчи. Для изменения формул производных показателей поднимать не нужно.
- **Статус и наличие результата разделены.** `status` (`pending`/`parsing`/`done`/`failed`) описывает обработку, `has_result` — есть ли данные. При пересчёте и его ошибке прежний результат сохраняется и показывается: `FailMatch` не трогает `match_players`. Итоги сессии строятся по `has_result = 1`.
- **Очередь** — таблица `matches`. `ClaimNextPending` идёт в порядке `has_result, id`, так что новые загрузки обрабатываются раньше пересчётов. При остановке сервиса матч остаётся `parsing`, при старте `ResetParsing` возвращает его в очередь, а не помечает `failed`.
- **Команды A/B:** A — команда, начавшая матч за CT. После смены сторон её сторона определяется по большинству её игроков. Счёт считается по победителям раундов. На `TeamState` не опираемся.
- **SteamID64 в JSON — строка**: в JavaScript number он теряет точность.
- **Фильтры и сортировка — в адресе страницы.** Сортировка: `sort` для главной таблицы, `ss`/`sm`/`sx` для разбивок профиля; значение по умолчанию в адрес не пишется. Шкала метрик сравнивает округлённое отображаемое значение.
- **Демки загружаются по одной** (`api.uploadDemo` с `AbortSignal`), очередь на клиенте строго последовательна — так порядок матчей совпадает с порядком файлов.
- **Схема БД меняется только новой миграцией** `internal/store/migrations/NNN_*.sql` с заполнением существующих строк. Старые миграции не править.
- **Хранилище демок** — интерфейс `ingest.Storage` (`Save`/`Open`/`Delete`), задел под S3/MinIO. Пока идёт разработка, демки хранятся без срока. Политику хранения для публичного деплоя (внешнее хранилище или удаление после парсинга) решить отдельным change.
- **Авторизации нет.** Сервис не открывать наружу без reverse proxy.

## Особенности демок платформ (выяснено сверкой с табло)

- **FastCup.** Первый раунд записи идёт до рестарта без события `MatchStart`, и игра его не засчитывает. Парсер сверяет число собранных раундов с `TotalRoundsPlayed` и отбрасывает лишние (`dropRestartedRounds`).
- **Общее.** События после конца матча (например, суицид на экране итогов) не учитываются: `MatchStartedChanged(false)` закрывает раунд. Флеш-ассисты засчитываются как ассисты, как в табло CS2 и FastCup.
- **Cybershoke** теряет статистику раунда, в котором игрок отключился, поэтому его табло не сходится само с собой. Под него парсер не подгоняем.
- **Старые бета-демки CS2 (2023)** могут не содержать событий урона, тогда ADR = 0.
- Известное ограничение: ADR игрока, который зашёл или вышел посреди матча, делится на все раунды матча.

Для отладки расхождений помогает временный тест, который печатает хронологию событий demoinfocs (RoundStart/RoundEnd/Kill с `TotalRoundsPlayed` и счётом). Создать его в `internal/zzdbg/`, после использования удалить.

## Данные и файлы пользователя

- `./data` в git не попадает: `cs2stats.db*`, `demos/<sha256>.dem`, `tmp/` (временные файлы загрузки сервиса).
- Пользователь кладёт исходные демки для проверки в `data/tmp/`. Не удалять и не перемещать их без спроса.
- Демки (`*.dem`, `*.dem.*`) не коммитить. Исключение — маленький тестовый `internal/ingest/testdata/sample.dem.bz2`.
- Эксперименты, которые меняют данные (поднятие версии, удаление демок), проводить на копии `data/` в отдельном контейнере или процессе, а не на рабочей базе.

## Проверка

- Unit-тесты на синтетике есть в каждом пакете; store, worker и api тестируются на временной SQLite.
- UI проверяется headless-Chromium из кэша Playwright (`~/.cache/ms-playwright/chromium-1140/chrome-linux/chrome`) через `playwright-core`. Скрипты держать в scratchpad, не в репозитории.
- Страницы матчей FastCup и Cybershoke рендерятся JavaScript'ом: табло для сверки снимать headless-браузером (`https://cs2.fastcup.net/matches/<id>`, `https://cybershoke.net/match/<id>`).
- Перед завершением change: `make test`, `npm run lint`, `openspec validate <change> --strict`, E2E в Docker.

## Подводные камни окружения

- zsh не разбивает `$var` на слова: `cmd $args` передаёт один аргумент. Аргументы писать явно или использовать массивы.
- `pkill -f <шаблон>` может убить собственный shell, если шаблон встречается в его командной строке.
- Сборка Docker иногда виснет на `load metadata for docker.io/library/golang:...`. Помогает отдельный `docker pull golang:1.27-alpine`.
- Обёртке таблицы с `overflow-x-auto` нужен `relative`: иначе `sr-only` (absolute) внутри неё растягивает всю страницу по горизонтали.
- `npm ci` внутри `docker build` может падать с `ETIMEDOUT` из-за сети WSL/Docker при многих параллельных загрузках. Для локальной проверки помогает копия Dockerfile с `ENV npm_config_maxsockets=2` перед `npm ci`.
- Контейнер работает от UID/GID 1000 (`user:` в compose), чтобы писать в смонтированный `./data`.
