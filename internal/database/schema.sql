-- Lighthouse Database Schema
-- Complete schema for SQLite database

-- =====================================================
-- CORE TABLES
-- =====================================================

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

-- =====================================================
-- TORRENTS
-- =====================================================

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

    -- Curation and dedup
    curation_status TEXT DEFAULT 'unknown',
    dedup_group_id TEXT,
    infohash_version TEXT DEFAULT 'v1',
    infohash_v2 TEXT,
    comment_count INTEGER DEFAULT 0,

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

-- Torrent comments (Kind 2004)
CREATE TABLE IF NOT EXISTS torrent_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT UNIQUE NOT NULL,
    infohash TEXT NOT NULL,
    torrent_event_id TEXT,
    author_pubkey TEXT NOT NULL,
    content TEXT NOT NULL,
    rating INTEGER,
    parent_id TEXT,
    root_id TEXT,
    mentions TEXT,
    created_at DATETIME NOT NULL,
    signature TEXT
);

-- =====================================================
-- WEB OF TRUST
-- =====================================================

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

-- =====================================================
-- CURATION
-- =====================================================

-- Rulesets table for versioned rule definitions
CREATE TABLE IF NOT EXISTS rulesets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ruleset_id TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('censoring', 'semantic')),
    version TEXT NOT NULL,
    hash TEXT NOT NULL,
    content TEXT NOT NULL,
    source TEXT,
    is_active BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deprecated_at DATETIME
);

-- Verification decisions from curators
CREATE TABLE IF NOT EXISTS verification_decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    decision_id TEXT UNIQUE NOT NULL,
    target_event_id TEXT NOT NULL,
    target_infohash TEXT NOT NULL,
    decision TEXT NOT NULL CHECK (decision IN ('accept', 'reject')),
    reason_codes TEXT,
    ruleset_type TEXT,
    ruleset_version TEXT,
    ruleset_hash TEXT,
    curator_pubkey TEXT NOT NULL,
    signature TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    processed_at DATETIME,
    aggregated_decision TEXT
);

-- Trust policies for curator management
CREATE TABLE IF NOT EXISTS trust_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    policy_id TEXT UNIQUE NOT NULL,
    version TEXT NOT NULL,
    hash TEXT NOT NULL,
    content TEXT NOT NULL,
    admin_pubkey TEXT NOT NULL,
    signature TEXT NOT NULL,
    effective_at DATETIME NOT NULL,
    expires_at DATETIME,
    is_current BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Curator trust (derived from policies)
CREATE TABLE IF NOT EXISTS curator_trust (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    curator_pubkey TEXT UNIQUE NOT NULL,
    alias TEXT,
    weight INTEGER DEFAULT 1,
    status TEXT DEFAULT 'approved' CHECK (status IN ('approved', 'revoked', 'pending')),
    approved_rulesets TEXT,
    approved_at DATETIME,
    revoked_at DATETIME,
    revoke_reason TEXT
);

-- Reports and appeals for moderation
CREATE TABLE IF NOT EXISTS reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    report_id TEXT UNIQUE NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('report', 'appeal')),
    target_event_id TEXT,
    target_infohash TEXT,
    category TEXT NOT NULL,
    evidence TEXT,
    scope TEXT,
    jurisdiction TEXT,
    reporter_pubkey TEXT NOT NULL,
    reporter_contact TEXT,
    signature TEXT,
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'acknowledged', 'investigating', 'resolved', 'rejected')),
    resolution TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    acknowledged_at DATETIME,
    resolved_at DATETIME,
    resolved_by TEXT,
    content TEXT
);

-- =====================================================
-- DEDUPLICATION
-- =====================================================

-- Deduplication groups
CREATE TABLE IF NOT EXISTS dedup_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id TEXT UNIQUE NOT NULL,
    canonical_torrent_id INTEGER,
    dedup_class TEXT CHECK (dedup_class IN ('exact', 'probable', 'related')),
    score INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (canonical_torrent_id) REFERENCES torrents(id) ON DELETE SET NULL
);

-- File hashes for probable deduplication
CREATE TABLE IF NOT EXISTS torrent_file_hashes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    torrent_id INTEGER NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    file_hash TEXT,
    FOREIGN KEY (torrent_id) REFERENCES torrents(id) ON DELETE CASCADE
);

-- =====================================================
-- RELAY MODE
-- =====================================================

-- Stored Nostr events (for relay mode)
CREATE TABLE IF NOT EXISTS relay_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id TEXT UNIQUE NOT NULL,
    pubkey TEXT NOT NULL,
    kind INTEGER NOT NULL,
    content TEXT NOT NULL,
    tags TEXT NOT NULL,
    sig TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    received_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- =====================================================
-- ACTIVITY LOG
-- =====================================================

-- Activity log for debugging and stats
CREATE TABLE IF NOT EXISTS activity_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,  -- 'torrent_added', 'relay_connected', 'trust_updated', etc.
    details TEXT,  -- JSON
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- =====================================================
-- INDEXES
-- =====================================================

CREATE INDEX IF NOT EXISTS idx_torrents_category ON torrents(category);
CREATE INDEX IF NOT EXISTS idx_torrents_trust_score ON torrents(trust_score DESC);
CREATE INDEX IF NOT EXISTS idx_torrents_first_seen ON torrents(first_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_torrents_info_hash ON torrents(info_hash);
CREATE INDEX IF NOT EXISTS idx_torrents_year ON torrents(year);
CREATE INDEX IF NOT EXISTS idx_torrents_curation ON torrents(curation_status);
CREATE INDEX IF NOT EXISTS idx_torrents_dedup_group ON torrents(dedup_group_id);
CREATE INDEX IF NOT EXISTS idx_torrent_uploads_torrent ON torrent_uploads(torrent_id);
CREATE INDEX IF NOT EXISTS idx_torrent_uploads_uploader ON torrent_uploads(uploader_npub);
CREATE INDEX IF NOT EXISTS idx_comments_torrent ON torrent_comments(infohash);
CREATE INDEX IF NOT EXISTS idx_comments_author ON torrent_comments(author_pubkey);
CREATE INDEX IF NOT EXISTS idx_comments_created ON torrent_comments(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_trust_follows_follower ON trust_follows(follower_npub);
CREATE INDEX IF NOT EXISTS idx_trust_follows_followed ON trust_follows(followed_npub);
CREATE INDEX IF NOT EXISTS idx_rulesets_active ON rulesets(is_active, type);
CREATE INDEX IF NOT EXISTS idx_rulesets_hash ON rulesets(hash);
CREATE INDEX IF NOT EXISTS idx_decisions_infohash ON verification_decisions(target_infohash);
CREATE INDEX IF NOT EXISTS idx_decisions_curator ON verification_decisions(curator_pubkey);
CREATE INDEX IF NOT EXISTS idx_decisions_created ON verification_decisions(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_decisions_decision ON verification_decisions(decision);
CREATE INDEX IF NOT EXISTS idx_policies_current ON trust_policies(is_current);
CREATE INDEX IF NOT EXISTS idx_policies_effective ON trust_policies(effective_at DESC);
CREATE INDEX IF NOT EXISTS idx_curator_status ON curator_trust(status);
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);
CREATE INDEX IF NOT EXISTS idx_reports_infohash ON reports(target_infohash);
CREATE INDEX IF NOT EXISTS idx_reports_created ON reports(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_dedup_groups_canonical ON dedup_groups(canonical_torrent_id);
CREATE INDEX IF NOT EXISTS idx_file_hashes_torrent ON torrent_file_hashes(torrent_id);
CREATE INDEX IF NOT EXISTS idx_file_hashes_hash ON torrent_file_hashes(file_hash);
CREATE INDEX IF NOT EXISTS idx_file_hashes_size ON torrent_file_hashes(file_size);
CREATE INDEX IF NOT EXISTS idx_relay_events_kind ON relay_events(kind);
CREATE INDEX IF NOT EXISTS idx_relay_events_pubkey ON relay_events(pubkey);
CREATE INDEX IF NOT EXISTS idx_relay_events_created ON relay_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_log_type ON activity_log(event_type);
CREATE INDEX IF NOT EXISTS idx_activity_log_created ON activity_log(created_at DESC);

-- =====================================================
-- DEFAULT SETTINGS
-- =====================================================

INSERT OR IGNORE INTO settings (key, value) VALUES
    ('setup_completed', 'false'),
    ('schema_version', '2'),
    ('indexer_enabled', 'true'),
    ('enrichment_enabled', 'true'),
    ('curator_enabled', 'false'),
    ('curator_mode', 'local'),
    ('aggregation_mode', 'any'),
    ('aggregation_quorum', '1'),
    ('relay_enabled', 'false'),
    ('dht_enabled', 'false');
