CREATE TABLE IF NOT EXISTS summary (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE,
    light_type TEXT
)