-- Lighthouse Database Schema
-- Version: 001_initial

-- Settings table for key-value configuration
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Identities table for Nostr identity management
CREATE TABLE IF NOT EXISTS identities (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    npub TEXT UNIQUE NOT NULL,
    nsec TEXT,  -- Encrypted, only for own identity
    name TEXT,
    is_own BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Relays table for Nostr relay management
CREATE TABLE IF NOT EXISTS relays (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    url TEXT UNIQUE NOT NULL,
    name TEXT,
    preset TEXT,  -- 'public', 'private', 'censorship-resistant'
    enabled BOOLEAN DEFAULT TRUE,
    status TEXT DEFAULT 'disconnected',
    last_connected_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Torrents table - main content storage
CREATE TABLE IF NOT EXISTS torrents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    info_hash TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    size INTEGER,  -- bytes
    category INTEGER,  -- Torznab category code
    seeders INTEGER DEFAULT 0,
    leechers INTEGER DEFAULT 0,
    magnet_uri TEXT NOT NULL,
    files TEXT,  -- JSON array of files

    -- Enriched metadata from TMDB/OMDB
    title TEXT,  -- Clean title
    year INTEGER,
    tmdb_id INTEGER,
    imdb_id TEXT,
    poster_url TEXT,
    backdrop_url TEXT,
    overview TEXT,
    genres TEXT,  -- JSON array
    rating REAL,

    -- Trust metrics
    trust_score INTEGER DEFAULT 0,
    upload_count INTEGER DEFAULT 1,  -- How many unique uploaders

    first_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Full-text search virtual table
CREATE VIRTUAL TABLE IF NOT EXISTS torrents_fts USING fts5(
    name,
    title,
    overview,
    content='torrents',
    content_rowid='id'
);

-- Triggers to keep FTS in sync with torrents table
CREATE TRIGGER IF NOT EXISTS torrents_ai AFTER INSERT ON torrents BEGIN
    INSERT INTO torrents_fts(rowid, name, title, overview)
    VALUES (new.id, new.name, new.title, new.overview);
END;

CREATE TRIGGER IF NOT EXISTS torrents_ad AFTER DELETE ON torrents BEGIN
    INSERT INTO torrents_fts(torrents_fts, rowid, name, title, overview)
    VALUES('delete', old.id, old.name, old.title, old.overview);
END;

CREATE TRIGGER IF NOT EXISTS torrents_au AFTER UPDATE ON torrents BEGIN
    INSERT INTO torrents_fts(torrents_fts, rowid, name, title, overview)
    VALUES('delete', old.id, old.name, old.title, old.overview);
    INSERT INTO torrents_fts(rowid, name, title, overview)
    VALUES (new.id, new.name, new.title, new.overview);
END;

-- Torrent uploads tracking (for deduplication & trust scoring)
CREATE TABLE IF NOT EXISTS torrent_uploads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    torrent_id INTEGER NOT NULL REFERENCES torrents(id) ON DELETE CASCADE,
    uploader_npub TEXT NOT NULL,
    nostr_event_id TEXT UNIQUE NOT NULL,
    relay_url TEXT,
    uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Web of Trust - Whitelist
CREATE TABLE IF NOT EXISTS trust_whitelist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    npub TEXT UNIQUE NOT NULL,
    alias TEXT,
    notes TEXT,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Web of Trust - Blacklist
CREATE TABLE IF NOT EXISTS trust_blacklist (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    npub TEXT UNIQUE NOT NULL,
    reason TEXT,
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Web of Trust - Follow graph
CREATE TABLE IF NOT EXISTS trust_follows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    follower_npub TEXT NOT NULL,
    followed_npub TEXT NOT NULL,
    depth INTEGER DEFAULT 1,  -- 1 = direct follow, 2 = friend of friend
    discovered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(follower_npub, followed_npub)
);

-- Activity log for debugging and stats
CREATE TABLE IF NOT EXISTS activity_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,  -- 'torrent_added', 'relay_connected', 'trust_updated', etc.
    details TEXT,  -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_torrents_category ON torrents(category);
CREATE INDEX IF NOT EXISTS idx_torrents_trust_score ON torrents(trust_score DESC);
CREATE INDEX IF NOT EXISTS idx_torrents_first_seen ON torrents(first_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_torrents_info_hash ON torrents(info_hash);
CREATE INDEX IF NOT EXISTS idx_torrents_year ON torrents(year);
CREATE INDEX IF NOT EXISTS idx_torrent_uploads_torrent ON torrent_uploads(torrent_id);
CREATE INDEX IF NOT EXISTS idx_torrent_uploads_uploader ON torrent_uploads(uploader_npub);
CREATE INDEX IF NOT EXISTS idx_trust_follows_follower ON trust_follows(follower_npub);
CREATE INDEX IF NOT EXISTS idx_trust_follows_followed ON trust_follows(followed_npub);
CREATE INDEX IF NOT EXISTS idx_activity_log_type ON activity_log(event_type);
CREATE INDEX IF NOT EXISTS idx_activity_log_created ON activity_log(created_at DESC);

-- Insert default settings
INSERT OR IGNORE INTO settings (key, value) VALUES
    ('setup_completed', 'false'),
    ('schema_version', '1'),
    ('indexer_enabled', 'true'),
    ('enrichment_enabled', 'true');
