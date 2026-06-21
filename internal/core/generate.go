package core

import (
	"database/sql"
	"fmt"
)

// maxRetries bounds the collision retry loop (SPEC §5.2).
const maxRetries = 100

// Generate produces one unique full name for the (project, lang) scope, prints
// it to stdout, and returns nil. On failure it returns an error (the caller
// maps it to exit code 1). See SPEC §5.2.
func Generate(db *sql.DB, project, lang string) error {
	// Pool sanity check.
	n, err := poolSize(db, lang)
	if err != nil {
		return fmt.Errorf("counting pool: %w", err)
	}
	if n < 2 {
		return fmt.Errorf(
			"language %q has only %d token(s); run --bootstrap to load names first",
			lang, n)
	}

	drawStmt, err := db.Prepare(
		`SELECT name FROM source_names WHERE language = ? ORDER BY random() LIMIT 2`,
	)
	if err != nil {
		return fmt.Errorf("preparing draw: %w", err)
	}
	defer drawStmt.Close()

	claimStmt, err := db.Prepare(
		`INSERT OR IGNORE INTO utilized_names (project_slug, language, full_name) VALUES (?, ?, ?)`,
	)
	if err != nil {
		return fmt.Errorf("preparing claim: %w", err)
	}
	defer claimStmt.Close()

	for attempt := 0; attempt < maxRetries; attempt++ {
		fullName, err := drawPair(drawStmt, lang)
		if err != nil {
			return err
		}

		res, err := claimStmt.Exec(project, lang, fullName)
		if err != nil {
			return fmt.Errorf("claiming combination: %w", err)
		}
		if affected, _ := res.RowsAffected(); affected == 1 {
			// Zero-overhead output: only the name on stdout (SPEC §2).
			fmt.Println(fullName)
			return nil
		}
		// affected == 0: already used for this scope -> retry.
	}

	// Failsafe: distinguish "exhausted" from "too small / bad luck" (SPEC §5.2).
	used, _ := usedCount(db, project, lang)
	maxCombos := n * (n - 1)
	if used >= maxCombos {
		return fmt.Errorf(
			"scope exhausted: all %s combinations for project=%q language=%q are used",
			formatInt(maxCombos), project, lang)
	}
	return fmt.Errorf(
		"maximum retries (%d) reached: pool of %s tokens is likely too small for the demand "+
			"(used %s of %s possible combinations)",
		maxRetries, formatInt(n), formatInt(used), formatInt(maxCombos))
}

// drawPair pulls two distinct tokens and joins them as "Name1 Name2".
func drawPair(stmt *sql.Stmt, lang string) (string, error) {
	rows, err := stmt.Query(lang)
	if err != nil {
		return "", fmt.Errorf("drawing names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", fmt.Errorf("scanning name: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("reading names: %w", err)
	}
	if len(names) < 2 {
		// Pool was validated as >= 2 before the loop, so this is unexpected.
		return "", fmt.Errorf("expected 2 names, got %d for language %q", len(names), lang)
	}

	return names[0] + " " + names[1], nil
}
