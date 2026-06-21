package core

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DBFile is the fixed database filename, always opened in the current
// working directory (see SPEC §3.1).
const DBFile = "random_names.db"

const schema = `
CREATE TABLE IF NOT EXISTS source_names (
    id       INTEGER PRIMARY KEY,
    name     TEXT NOT NULL,
    language TEXT NOT NULL,
    UNIQUE (language, name)
);

CREATE INDEX IF NOT EXISTS idx_source_names_lang
    ON source_names (language);

CREATE TABLE IF NOT EXISTS utilized_names (
    project_slug TEXT NOT NULL,
    language     TEXT NOT NULL,
    full_name    TEXT NOT NULL,
    PRIMARY KEY (project_slug, language, full_name)
);
`

// Open opens (or creates) random_names.db in the current working directory,
// applies the runtime pragmas, and ensures the schema exists.
func Open() (*sql.DB, error) {
	// WAL + busy_timeout make concurrent invocations safe (SPEC §3.1, §5.3).
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", DBFile)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// A single connection avoids surprises with per-connection pragmas and is
	// plenty for a short-lived CLI invocation.
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return db, nil
}

// poolSize returns how many source tokens exist for the given language.
func poolSize(db *sql.DB, lang string) (int64, error) {
	var n int64
	err := db.QueryRow(
		`SELECT COUNT(*) FROM source_names WHERE language = ?`, lang,
	).Scan(&n)
	return n, err
}

// usedCount returns how many combinations were already produced for the scope.
func usedCount(db *sql.DB, project, lang string) (int64, error) {
	var n int64
	err := db.QueryRow(
		`SELECT COUNT(*) FROM utilized_names WHERE project_slug = ? AND language = ?`,
		project, lang,
	).Scan(&n)
	return n, err
}
