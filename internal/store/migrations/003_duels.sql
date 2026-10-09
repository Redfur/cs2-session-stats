-- личные дуэли: сколько раз killer убил victim в матче; хранятся все пары соперников, включая нули
CREATE TABLE match_duels (
    match_id  INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    killer_id INTEGER NOT NULL,
    victim_id INTEGER NOT NULL,
    kills     INTEGER NOT NULL CHECK (kills >= 0),
    PRIMARY KEY (match_id, killer_id, victim_id)
);

-- result_version: версия обработки, которой получен сохранённый результат (в отличие от processed_version,
-- не меняется при неудачной попытке). Дуэли посчитаны у матчей с has_result = 1 AND result_version >= 2.
ALTER TABLE matches ADD COLUMN result_version INTEGER NOT NULL DEFAULT 0;

UPDATE matches SET result_version = 1 WHERE has_result = 1;
