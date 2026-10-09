# Tasks

## 1. API

- [ ] 1.1 `internal/api/duels.go`: выбор `beats`/`losesTo` по доле (перекрёстное умножение, граница 50%, пары 0:0 исключены, все равные, порядок K+D ↓ → ник → SteamID) вместо `maxBy`. Тесты в `internal/api/duels_test.go`: доля важнее объёма (30:25 vs 12:7), равенство 2:1 и 4:2 с порядком, все пары выиграны → `losesTo` пуст, 1:0 → 100%, 5:5 не попадает никуда, все 0:0; `go test ./internal/api/` проходит.

## 2. Интерфейс

- [ ] 2.1 `web/src/api.ts`, `web/src/components/Duels.tsx`, `web/src/components/ui/Rivals.tsx`: поля `beats`/`losesTo`, заголовки «Выигрывает дуэли у» / «Проигрывает дуэли», доля через `duelPct` в строке, пустая колонка с текстом «Нет пар с долей выше/ниже 50%». Проверить headless-Chromium на копии `data/` (1440 и 390 px): у perlov строки с долей, длинные ники не ломают строку, нет горизонтальной прокрутки.

## 3. Документация и проверки

- [ ] 3.1 README, раздел «Личные дуэли»: правило выбора по доле и названия показателей; текст соответствует коду.
- [ ] 3.2 `make test`, `npm run lint`, `openspec validate rivals-by-share --strict`, E2E в Docker на копии `data/`: профиль показывает «Соперников» по доле.

## Workflow follow-up

- `/opsx:archive rivals-by-share` с синхронизацией спек.
