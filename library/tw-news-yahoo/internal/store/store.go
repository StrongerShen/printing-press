package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS articles (
	uuid          TEXT PRIMARY KEY,
	url           TEXT NOT NULL,
	headline      TEXT NOT NULL,
	description   TEXT,
	keywords      TEXT,
	entities      TEXT,
	provider      TEXT,
	date_published INTEGER,
	date_modified  INTEGER,
	fetched_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS meta (
	key   TEXT PRIMARY KEY,
	value TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS articles_fts USING fts5(
	uuid UNINDEXED,
	headline,
	description,
	keywords,
	entities,
	content='articles',
	content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS articles_ai AFTER INSERT ON articles BEGIN
	INSERT INTO articles_fts(rowid, uuid, headline, description, keywords, entities)
	VALUES (new.rowid, new.uuid, new.headline, new.description, new.keywords, new.entities);
END;

CREATE TRIGGER IF NOT EXISTS articles_ad AFTER DELETE ON articles BEGIN
	INSERT INTO articles_fts(articles_fts, rowid, uuid, headline, description, keywords, entities)
	VALUES ('delete', old.rowid, old.uuid, old.headline, old.description, old.keywords, old.entities);
END;

CREATE TRIGGER IF NOT EXISTS articles_au AFTER UPDATE ON articles BEGIN
	INSERT INTO articles_fts(articles_fts, rowid, uuid, headline, description, keywords, entities)
	VALUES ('delete', old.rowid, old.uuid, old.headline, old.description, old.keywords, old.entities);
	INSERT INTO articles_fts(rowid, uuid, headline, description, keywords, entities)
	VALUES (new.rowid, new.uuid, new.headline, new.description, new.keywords, new.entities);
END;
`

type Article struct {
	UUID          string
	URL           string
	Headline      string
	Description   string
	Keywords      []string
	Entities      []string
	Provider      string
	DatePublished time.Time
	DateModified  time.Time
	FetchedAt     time.Time
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Upsert(a *Article) error {
	kw, _ := json.Marshal(a.Keywords)
	ent, _ := json.Marshal(a.Entities)
	_, err := s.db.Exec(`
		INSERT INTO articles (uuid, url, headline, description, keywords, entities, provider, date_published, date_modified, fetched_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(uuid) DO UPDATE SET
			headline=excluded.headline,
			description=excluded.description,
			keywords=excluded.keywords,
			entities=excluded.entities,
			date_modified=excluded.date_modified,
			fetched_at=excluded.fetched_at
	`,
		a.UUID, a.URL, a.Headline, a.Description,
		string(kw), string(ent), a.Provider,
		a.DatePublished.Unix(), a.DateModified.Unix(),
		a.FetchedAt.Unix(),
	)
	return err
}

func (s *Store) Exists(uuid string) bool {
	var n int
	s.db.QueryRow("SELECT COUNT(1) FROM articles WHERE uuid=?", uuid).Scan(&n)
	return n > 0
}

func (s *Store) ExistsByURL(articleURL string) bool {
	var n int
	s.db.QueryRow("SELECT COUNT(1) FROM articles WHERE url=?", articleURL).Scan(&n)
	return n > 0
}

type Filter struct {
	Keyword string
	From    time.Time
	To      time.Time
	Limit   int
}

func (s *Store) List(f Filter) ([]*Article, error) {
	var conds []string
	var args []any

	if f.Keyword != "" {
		conds = append(conds, "(headline LIKE ? OR description LIKE ? OR keywords LIKE ? OR entities LIKE ?)")
		pat := "%" + f.Keyword + "%"
		args = append(args, pat, pat, pat, pat)
	}
	if !f.From.IsZero() {
		conds = append(conds, "date_published >= ?")
		args = append(args, f.From.Unix())
	}
	if !f.To.IsZero() {
		conds = append(conds, "date_published <= ?")
		args = append(args, f.To.Unix())
	}

	q := "SELECT uuid, url, headline, description, keywords, entities, provider, date_published, date_modified, fetched_at FROM articles"
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY date_published DESC"
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	return s.queryArticles(q, args...)
}

func (s *Store) Search(query string, limit int) ([]*Article, error) {
	q := `
		SELECT a.uuid, a.url, a.headline, a.description, a.keywords, a.entities, a.provider,
			a.date_published, a.date_modified, a.fetched_at
		FROM articles_fts f
		JOIN articles a ON a.rowid = f.rowid
		WHERE articles_fts MATCH ?
		ORDER BY rank
	`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	return s.queryArticles(q, query)
}

func (s *Store) queryArticles(q string, args ...any) ([]*Article, error) {
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Article
	for rows.Next() {
		var a Article
		var kwJSON, entJSON string
		var pubUnix, modUnix, fetchUnix int64
		if err := rows.Scan(&a.UUID, &a.URL, &a.Headline, &a.Description,
			&kwJSON, &entJSON, &a.Provider,
			&pubUnix, &modUnix, &fetchUnix); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(kwJSON), &a.Keywords)
		json.Unmarshal([]byte(entJSON), &a.Entities)
		a.DatePublished = time.Unix(pubUnix, 0).UTC()
		a.DateModified = time.Unix(modUnix, 0).UTC()
		a.FetchedAt = time.Unix(fetchUnix, 0).UTC()
		out = append(out, &a)
	}
	return out, rows.Err()
}

func (s *Store) Count() int {
	var n int
	s.db.QueryRow("SELECT COUNT(1) FROM articles").Scan(&n)
	return n
}

func (s *Store) LastSyncTime() time.Time {
	var v string
	s.db.QueryRow("SELECT value FROM meta WHERE key='last_sync'").Scan(&v)
	if v == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339, v)
	return t
}

func (s *Store) SetLastSyncTime(t time.Time) {
	s.db.Exec("INSERT OR REPLACE INTO meta(key,value) VALUES('last_sync',?)", t.UTC().Format(time.RFC3339))
}

func (s *Store) NewestPublishedAt() time.Time {
	var unix int64
	s.db.QueryRow("SELECT MAX(date_published) FROM articles").Scan(&unix)
	if unix == 0 {
		return time.Time{}
	}
	return time.Unix(unix, 0).UTC()
}

// DailyCount returns article counts grouped by date (YYYY-MM-DD in Asia/Taipei).
func (s *Store) DailyCount(from, to time.Time) (map[string]int, error) {
	q := `SELECT DATE(date_published, 'unixepoch', '+8 hours') as d, COUNT(1)
	      FROM articles WHERE date_published >= ? AND date_published <= ?
	      GROUP BY d ORDER BY d`
	rows, err := s.db.Query(q, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]int{}
	for rows.Next() {
		var d string
		var n int
		rows.Scan(&d, &n)
		result[d] = n
	}
	return result, rows.Err()
}
