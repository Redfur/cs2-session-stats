-- загрузки по ссылке на матч платформы: очередь фонового скачивания демок
CREATE TABLE imports (
    id          INTEGER PRIMARY KEY,
    session_id  INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    url         TEXT    NOT NULL,                   -- нормализованная ссылка на страницу матча
    platform    TEXT    NOT NULL,                   -- fastcup | cybershoke
    external_id TEXT    NOT NULL,                   -- номер матча на платформе
    status      TEXT    NOT NULL DEFAULT 'queued',  -- queued | downloading | done | failed
    error       TEXT    NOT NULL DEFAULT '',
    bytes_done  INTEGER NOT NULL DEFAULT 0,
    bytes_total INTEGER,                            -- NULL, если размер неизвестен
    results     TEXT    NOT NULL DEFAULT '[]',      -- JSON: итог по каждой карте
    created_at  TEXT    NOT NULL,
    finished_at TEXT
);

CREATE INDEX imports_source ON imports(platform, external_id);
CREATE INDEX imports_status ON imports(status);

-- происхождение матча; при удалении загрузки матч остаётся
ALTER TABLE matches ADD COLUMN import_id INTEGER REFERENCES imports(id) ON DELETE SET NULL;
-- время начала карты у платформы (UTC, 2006-01-02T15:04:05Z); NULL у матчей из файлов
ALTER TABLE matches ADD COLUMN played_at TEXT;
-- источник матча: платформа, номер матча на ней и номер карты в серии. Нужны для проверки
-- «уже загружен» (переживает удаление загрузки) и для порядка при равном played_at
ALTER TABLE matches ADD COLUMN source_platform TEXT;
ALTER TABLE matches ADD COLUMN source_number INTEGER;
ALTER TABLE matches ADD COLUMN map_number INTEGER;

CREATE INDEX matches_import ON matches(import_id);
CREATE INDEX matches_source ON matches(source_platform, source_number);
