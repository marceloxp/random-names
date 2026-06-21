package core

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
)

// Bootstrap ingests names from a plain-text file (one name per line) into
// source_names, tagged with lang. See SPEC §5.1.
func Bootstrap(db *sql.DB, file, lang string) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback() // no-op after a successful Commit

	stmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO source_names (name, language) VALUES (?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("preparing insert: %w", err)
	}
	defer stmt.Close()

	var read, inserted, skipped int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		read++
		name := NormalizeToken(scanner.Text())
		if name == "" {
			skipped++
			continue
		}

		res, err := stmt.Exec(name, lang)
		if err != nil {
			return fmt.Errorf("inserting %q: %w", name, err)
		}
		if n, _ := res.RowsAffected(); n == 1 {
			inserted++
		} else {
			skipped++ // duplicate (already present for this language)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	fmt.Printf("Bootstrap complete: %s read, %s inserted, %s skipped (language=%s)\n",
		formatInt(read), formatInt(inserted), formatInt(skipped), lang)
	return nil
}
