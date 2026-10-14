-- группы компаний: у каждой свои сессии, статистика и рейтинг.
-- Существующие сессии переходят в группу 1 «Основная»; на пустой базе она тоже создаётся.
-- group_id допускает NULL только из-за ограничения ADD COLUMN с REFERENCES при foreign_keys=1:
-- CreateSession всегда задаёт группу.
CREATE TABLE groups (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL
);
INSERT INTO groups (id, name, created_at) VALUES (1, 'Основная', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));
ALTER TABLE sessions ADD COLUMN group_id INTEGER REFERENCES groups(id);
UPDATE sessions SET group_id = 1;
CREATE INDEX sessions_group ON sessions(group_id, date, id);
