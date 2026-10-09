CREATE TABLE sessions (
    id         INTEGER PRIMARY KEY,
    date       TEXT NOT NULL, -- YYYY-MM-DD
    title      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE TABLE matches (
    id            INTEGER PRIMARY KEY,
    session_id    INTEGER NOT NULL REFERENCES sessions(id),
    ordinal       INTEGER NOT NULL,
    sha256        TEXT NOT NULL UNIQUE,
    original_name TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending', -- pending | parsing | done | failed
    error         TEXT NOT NULL DEFAULT '',
    map           TEXT NOT NULL DEFAULT '',
    rounds        INTEGER NOT NULL DEFAULT 0,
    score_a       INTEGER NOT NULL DEFAULT 0,
    score_b       INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    parsed_at     TEXT,
    UNIQUE (session_id, ordinal)
);

CREATE INDEX matches_status ON matches(status);

CREATE TABLE match_players (
    match_id       INTEGER NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
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
