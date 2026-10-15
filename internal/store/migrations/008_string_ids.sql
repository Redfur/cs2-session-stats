-- строковые неугадываемые id групп, сессий, матчей и загрузок (g_…, s_…, m_…, imp_…).
-- Таблицы соответствия id_map_* (old → new) заполняет Go-шаг stringIDsBefore в этой же транзакции.
-- seq — внутренний порядок вставки (для существующих строк — старый id): по нему сортируются
-- списки и очереди; наружу он не отдаётся.
--
-- Пересборка без выключения foreign_keys: старые таблицы переименовываются в *_old (ссылки
-- дочерних таблиц переписываются на *_old), новые создаются под итоговыми именами и заполняются
-- от родителей к детям, затем старые удаляются от детей к родителям. Каскады при DROP старых
-- таблиц задевают только старые таблицы.

ALTER TABLE match_clutches RENAME TO match_clutches_old;
ALTER TABLE match_kills RENAME TO match_kills_old;
ALTER TABLE match_rounds RENAME TO match_rounds_old;
ALTER TABLE match_player_weapons RENAME TO match_player_weapons_old;
ALTER TABLE match_player_ext RENAME TO match_player_ext_old;
ALTER TABLE match_duels RENAME TO match_duels_old;
ALTER TABLE match_players RENAME TO match_players_old;
ALTER TABLE matches RENAME TO matches_old;
ALTER TABLE imports RENAME TO imports_old;
ALTER TABLE sessions RENAME TO sessions_old;
ALTER TABLE groups RENAME TO groups_old;

CREATE TABLE groups (
    id         TEXT PRIMARY KEY,
    seq        INTEGER NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    seq        INTEGER NOT NULL UNIQUE,
    group_id   TEXT NOT NULL REFERENCES groups(id),
    date       TEXT NOT NULL, -- YYYY-MM-DD
    title      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE imports (
    id          TEXT PRIMARY KEY,
    seq         INTEGER NOT NULL UNIQUE,
    session_id  TEXT    NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
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

CREATE TABLE matches (
    id                TEXT PRIMARY KEY,
    seq               INTEGER NOT NULL UNIQUE,
    session_id        TEXT NOT NULL REFERENCES sessions(id),
    ordinal           INTEGER NOT NULL,
    sha256            TEXT NOT NULL UNIQUE,
    original_name     TEXT NOT NULL,
    status            TEXT NOT NULL DEFAULT 'pending', -- pending | parsing | done | failed
    error             TEXT NOT NULL DEFAULT '',
    map               TEXT NOT NULL DEFAULT '',
    rounds            INTEGER NOT NULL DEFAULT 0,
    score_a           INTEGER NOT NULL DEFAULT 0,
    score_b           INTEGER NOT NULL DEFAULT 0,
    created_at        TEXT NOT NULL,
    parsed_at         TEXT,
    has_result        INTEGER NOT NULL DEFAULT 0,
    processed_version INTEGER NOT NULL DEFAULT 0,
    result_version    INTEGER NOT NULL DEFAULT 0,
    import_id         TEXT REFERENCES imports(id) ON DELETE SET NULL,
    played_at         TEXT,
    source_platform   TEXT,
    source_number     INTEGER,
    map_number        INTEGER,
    first_round       INTEGER NOT NULL DEFAULT 1,
    restored_round    INTEGER NOT NULL DEFAULT 0,
    has_damage_events INTEGER NOT NULL DEFAULT 1,
    has_flash_events  INTEGER NOT NULL DEFAULT 1,
    duration_ms       INTEGER NOT NULL DEFAULT 0,
    UNIQUE (session_id, ordinal)
);

CREATE TABLE match_players (
    match_id       TEXT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    steam_id       INTEGER NOT NULL,
    name           TEXT NOT NULL,
    team           TEXT NOT NULL, -- A | B
    result         TEXT NOT NULL, -- win | loss | draw
    rounds         INTEGER NOT NULL,
    kills          INTEGER NOT NULL,
    deaths         INTEGER NOT NULL,
    assists        INTEGER NOT NULL,
    hs_kills       INTEGER NOT NULL,
    damage         INTEGER NOT NULL,
    kast_rounds    INTEGER NOT NULL,
    k1             INTEGER NOT NULL,
    k2             INTEGER NOT NULL,
    k3             INTEGER NOT NULL,
    k4             INTEGER NOT NULL,
    k5             INTEGER NOT NULL,
    opening_kills  INTEGER NOT NULL,
    opening_deaths INTEGER NOT NULL,
    PRIMARY KEY (match_id, steam_id)
);

CREATE TABLE match_duels (
    match_id  TEXT    NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    killer_id INTEGER NOT NULL,
    victim_id INTEGER NOT NULL,
    kills     INTEGER NOT NULL CHECK (kills >= 0),
    PRIMARY KEY (match_id, killer_id, victim_id)
);

CREATE TABLE match_player_ext (
    match_id        TEXT    NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    steam_id        INTEGER NOT NULL,
    rounds          INTEGER NOT NULL,
    traded_deaths   INTEGER NOT NULL,
    trade_kills     INTEGER NOT NULL,
    damage_dealt    INTEGER NOT NULL,
    damage_taken    INTEGER NOT NULL,
    flash_assists   INTEGER NOT NULL,
    damage_assists  INTEGER NOT NULL,
    rounds_t        INTEGER NOT NULL,
    rounds_ct       INTEGER NOT NULL,
    survived_t      INTEGER NOT NULL,
    survived_ct     INTEGER NOT NULL,
    alive_ms_t      INTEGER NOT NULL,
    alive_ms_ct     INTEGER NOT NULL,
    he_damage       INTEGER NOT NULL,
    fire_damage     INTEGER NOT NULL,
    flashed         INTEGER NOT NULL,
    smokes          INTEGER NOT NULL,
    he_kills        INTEGER NOT NULL,
    fire_kills      INTEGER NOT NULL,
    impact_kills    INTEGER NOT NULL,
    smoke_kills     INTEGER NOT NULL,
    wallbang_kills  INTEGER NOT NULL,
    noscope_kills   INTEGER NOT NULL,
    blind_kills     INTEGER NOT NULL,
    distance_sum    REAL    NOT NULL,
    distance_kills  INTEGER NOT NULL,
    plants          INTEGER NOT NULL,
    plant_starts    INTEGER NOT NULL,
    defuses         INTEGER NOT NULL,
    defuse_starts   INTEGER NOT NULL,
    PRIMARY KEY (match_id, steam_id)
);

CREATE TABLE match_player_weapons (
    match_id TEXT    NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    steam_id INTEGER NOT NULL,
    weapon   TEXT    NOT NULL, -- код оружия (ak47, m4a1_silencer, knife…) или исходное имя
    kills    INTEGER NOT NULL,
    hs_kills INTEGER NOT NULL,
    damage   INTEGER NOT NULL,
    shots    INTEGER NOT NULL,
    hits     INTEGER NOT NULL,
    hs_hits  INTEGER NOT NULL,
    PRIMARY KEY (match_id, steam_id, weapon)
);

CREATE TABLE match_rounds (
    match_id TEXT    NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    number   INTEGER NOT NULL,
    winner   TEXT    NOT NULL, -- A | B
    side_a   TEXT    NOT NULL, -- CT | T
    reason   TEXT    NOT NULL, -- elimination | bomb | defuse | time | other; пусто у восстановленного
    start_ms INTEGER NOT NULL, -- конец freeze time от начала демки; 0 — неизвестен
    end_ms   INTEGER NOT NULL, -- RoundEnd; 0 — не наблюдался
    restored INTEGER NOT NULL,
    equip_a  INTEGER NOT NULL DEFAULT 0,
    equip_b  INTEGER NOT NULL DEFAULT 0,
    plant_ms INTEGER NOT NULL DEFAULT 0, -- от начала демки; 0 — бомбу не установили
    PRIMARY KEY (match_id, number)
);

CREATE TABLE match_kills (
    match_id       TEXT    NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    round          INTEGER NOT NULL,
    seq            INTEGER NOT NULL,
    time_ms        INTEGER NOT NULL, -- от начала демки
    killer_id      INTEGER NOT NULL, -- 0 — мир
    victim_id      INTEGER NOT NULL,
    assister_id    INTEGER NOT NULL,
    weapon         TEXT    NOT NULL,
    headshot       INTEGER NOT NULL,
    flash_assist   INTEGER NOT NULL,
    through_smoke  INTEGER NOT NULL,
    wallbang       INTEGER NOT NULL,
    noscope        INTEGER NOT NULL,
    attacker_blind INTEGER NOT NULL,
    distance       REAL    NOT NULL,
    trade          INTEGER NOT NULL, -- убийство в размен
    PRIMARY KEY (match_id, round, seq)
);

CREATE TABLE match_clutches (
    match_id TEXT    NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    round    INTEGER NOT NULL,
    steam_id INTEGER NOT NULL,
    vs       INTEGER NOT NULL CHECK (vs BETWEEN 1 AND 5),
    outcome  TEXT    NOT NULL, -- win | loss | draw | unknown
    PRIMARY KEY (match_id, round, steam_id)
);

-- LEFT JOIN: ссылка без соответствия даёт NULL и падает на NOT NULL, а не теряет строку молча
INSERT INTO groups (id, seq, name, created_at)
SELECT g.new, o.id, o.name, o.created_at
FROM groups_old o LEFT JOIN id_map_groups g ON g.old = o.id;

INSERT INTO sessions (id, seq, group_id, date, title, created_at)
SELECT s.new, o.id, g.new, o.date, o.title, o.created_at
FROM sessions_old o LEFT JOIN id_map_sessions s ON s.old = o.id LEFT JOIN id_map_groups g ON g.old = o.group_id;

INSERT INTO imports (id, seq, session_id, url, platform, external_id, status, error, bytes_done, bytes_total,
    results, created_at, finished_at)
SELECT i.new, o.id, s.new, o.url, o.platform, o.external_id, o.status, o.error, o.bytes_done, o.bytes_total,
    o.results, o.created_at, o.finished_at
FROM imports_old o LEFT JOIN id_map_imports i ON i.old = o.id LEFT JOIN id_map_sessions s ON s.old = o.session_id;

INSERT INTO matches (id, seq, session_id, ordinal, sha256, original_name, status, error, map, rounds, score_a, score_b,
    created_at, parsed_at, has_result, processed_version, result_version, import_id, played_at, source_platform,
    source_number, map_number, first_round, restored_round, has_damage_events, has_flash_events, duration_ms)
SELECT m.new, o.id, s.new, o.ordinal, o.sha256, o.original_name, o.status, o.error, o.map, o.rounds, o.score_a, o.score_b,
    o.created_at, o.parsed_at, o.has_result, o.processed_version, o.result_version, i.new, o.played_at, o.source_platform,
    o.source_number, o.map_number, o.first_round, o.restored_round, o.has_damage_events, o.has_flash_events, o.duration_ms
FROM matches_old o LEFT JOIN id_map_matches m ON m.old = o.id LEFT JOIN id_map_sessions s ON s.old = o.session_id
    LEFT JOIN id_map_imports i ON i.old = o.import_id;

INSERT INTO match_players (match_id, steam_id, name, team, result, rounds, kills, deaths, assists, hs_kills, damage,
    kast_rounds, k1, k2, k3, k4, k5, opening_kills, opening_deaths)
SELECT m.new, o.steam_id, o.name, o.team, o.result, o.rounds, o.kills, o.deaths, o.assists, o.hs_kills, o.damage,
    o.kast_rounds, o.k1, o.k2, o.k3, o.k4, o.k5, o.opening_kills, o.opening_deaths
FROM match_players_old o LEFT JOIN id_map_matches m ON m.old = o.match_id;

INSERT INTO match_duels (match_id, killer_id, victim_id, kills)
SELECT m.new, o.killer_id, o.victim_id, o.kills
FROM match_duels_old o LEFT JOIN id_map_matches m ON m.old = o.match_id;

INSERT INTO match_player_ext (match_id, steam_id, rounds, traded_deaths, trade_kills, damage_dealt, damage_taken,
    flash_assists, damage_assists, rounds_t, rounds_ct, survived_t, survived_ct, alive_ms_t, alive_ms_ct, he_damage,
    fire_damage, flashed, smokes, he_kills, fire_kills, impact_kills, smoke_kills, wallbang_kills, noscope_kills,
    blind_kills, distance_sum, distance_kills, plants, plant_starts, defuses, defuse_starts)
SELECT m.new, o.steam_id, o.rounds, o.traded_deaths, o.trade_kills, o.damage_dealt, o.damage_taken,
    o.flash_assists, o.damage_assists, o.rounds_t, o.rounds_ct, o.survived_t, o.survived_ct, o.alive_ms_t, o.alive_ms_ct, o.he_damage,
    o.fire_damage, o.flashed, o.smokes, o.he_kills, o.fire_kills, o.impact_kills, o.smoke_kills, o.wallbang_kills, o.noscope_kills,
    o.blind_kills, o.distance_sum, o.distance_kills, o.plants, o.plant_starts, o.defuses, o.defuse_starts
FROM match_player_ext_old o LEFT JOIN id_map_matches m ON m.old = o.match_id;

INSERT INTO match_player_weapons (match_id, steam_id, weapon, kills, hs_kills, damage, shots, hits, hs_hits)
SELECT m.new, o.steam_id, o.weapon, o.kills, o.hs_kills, o.damage, o.shots, o.hits, o.hs_hits
FROM match_player_weapons_old o LEFT JOIN id_map_matches m ON m.old = o.match_id;

INSERT INTO match_rounds (match_id, number, winner, side_a, reason, start_ms, end_ms, restored, equip_a, equip_b, plant_ms)
SELECT m.new, o.number, o.winner, o.side_a, o.reason, o.start_ms, o.end_ms, o.restored, o.equip_a, o.equip_b, o.plant_ms
FROM match_rounds_old o LEFT JOIN id_map_matches m ON m.old = o.match_id;

INSERT INTO match_kills (match_id, round, seq, time_ms, killer_id, victim_id, assister_id, weapon, headshot,
    flash_assist, through_smoke, wallbang, noscope, attacker_blind, distance, trade)
SELECT m.new, o.round, o.seq, o.time_ms, o.killer_id, o.victim_id, o.assister_id, o.weapon, o.headshot,
    o.flash_assist, o.through_smoke, o.wallbang, o.noscope, o.attacker_blind, o.distance, o.trade
FROM match_kills_old o LEFT JOIN id_map_matches m ON m.old = o.match_id;

INSERT INTO match_clutches (match_id, round, steam_id, vs, outcome)
SELECT m.new, o.round, o.steam_id, o.vs, o.outcome
FROM match_clutches_old o LEFT JOIN id_map_matches m ON m.old = o.match_id;

DROP TABLE match_clutches_old;
DROP TABLE match_kills_old;
DROP TABLE match_rounds_old;
DROP TABLE match_player_weapons_old;
DROP TABLE match_player_ext_old;
DROP TABLE match_duels_old;
DROP TABLE match_players_old;
DROP TABLE matches_old;
DROP TABLE imports_old;
DROP TABLE sessions_old;
DROP TABLE groups_old;

DROP TABLE id_map_groups;
DROP TABLE id_map_sessions;
DROP TABLE id_map_matches;
DROP TABLE id_map_imports;

CREATE INDEX sessions_group ON sessions(group_id, date, seq);
CREATE INDEX imports_source ON imports(platform, external_id);
CREATE INDEX imports_status ON imports(status);
CREATE INDEX matches_status ON matches(status);
CREATE INDEX matches_import ON matches(import_id);
CREATE INDEX matches_source ON matches(source_platform, source_number);
CREATE INDEX match_player_ext_player ON match_player_ext(steam_id);
CREATE INDEX match_player_weapons_player ON match_player_weapons(steam_id);
CREATE INDEX match_clutches_player ON match_clutches(steam_id);
