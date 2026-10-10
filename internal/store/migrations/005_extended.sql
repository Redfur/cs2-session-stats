-- расширенная статистика из демок. Посчитана у матчей с has_result = 1 AND result_version >= 3;
-- у существующих матчей result_version меньше, поэтому до пересчёта она «не посчитана».

-- признаки записи демки
ALTER TABLE matches ADD COLUMN first_round INTEGER NOT NULL DEFAULT 1;       -- номер первого записанного раунда
ALTER TABLE matches ADD COLUMN restored_round INTEGER NOT NULL DEFAULT 0;    -- победитель последнего раунда восстановлен из счёта
ALTER TABLE matches ADD COLUMN has_damage_events INTEGER NOT NULL DEFAULT 1; -- в демке есть события урона
ALTER TABLE matches ADD COLUMN has_flash_events INTEGER NOT NULL DEFAULT 1;  -- в демке есть события ослепления
ALTER TABLE matches ADD COLUMN duration_ms INTEGER NOT NULL DEFAULT 0;       -- от начала первого раунда до конца последнего

-- расширенные счётчики игрока матча (без восстановленного раунда)
CREATE TABLE match_player_ext (
    match_id        INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
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

-- оружие игрока матча, без гранат и огня
CREATE TABLE match_player_weapons (
    match_id INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
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

-- хронология раундов
CREATE TABLE match_rounds (
    match_id INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    number   INTEGER NOT NULL,
    winner   TEXT    NOT NULL, -- A | B
    side_a   TEXT    NOT NULL, -- CT | T
    reason   TEXT    NOT NULL, -- elimination | bomb | defuse | time | other; пусто у восстановленного
    start_ms INTEGER NOT NULL, -- конец freeze time от начала демки; 0 — неизвестен
    end_ms   INTEGER NOT NULL, -- RoundEnd; 0 — не наблюдался
    restored INTEGER NOT NULL,
    PRIMARY KEY (match_id, number)
);

-- убийства раундов (ход раунда), кроме восстановленного раунда
CREATE TABLE match_kills (
    match_id       INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
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

-- попытки клатча
CREATE TABLE match_clutches (
    match_id INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    round    INTEGER NOT NULL,
    steam_id INTEGER NOT NULL,
    vs       INTEGER NOT NULL CHECK (vs BETWEEN 1 AND 5),
    outcome  TEXT    NOT NULL, -- win | loss | draw | unknown
    PRIMARY KEY (match_id, round, steam_id)
);

CREATE INDEX match_player_ext_player ON match_player_ext(steam_id);
CREATE INDEX match_player_weapons_player ON match_player_weapons(steam_id);
CREATE INDEX match_clutches_player ON match_clutches(steam_id);
