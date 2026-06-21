package core

import (
	"database/sql"
	"fmt"
)

// scopeStats holds the computed figures for a (project, lang) scope.
type scopeStats struct {
	pool      int64
	maxCombos int64
	used      int64
	remaining int64
}

func computeStats(db *sql.DB, project, lang string) (scopeStats, error) {
	var s scopeStats
	var err error

	if s.pool, err = poolSize(db, lang); err != nil {
		return s, fmt.Errorf("counting pool: %w", err)
	}
	if s.used, err = usedCount(db, project, lang); err != nil {
		return s, fmt.Errorf("counting used: %w", err)
	}
	if s.pool >= 2 {
		s.maxCombos = s.pool * (s.pool - 1)
	}
	s.remaining = s.maxCombos - s.used
	if s.remaining < 0 {
		s.remaining = 0
	}
	return s, nil
}

// Count prints only the number of remaining combinations for the scope (SPEC §5.4).
func Count(db *sql.DB, project, lang string) error {
	s, err := computeStats(db, project, lang)
	if err != nil {
		return err
	}
	fmt.Println(s.remaining)
	return nil
}

// Stats prints a human-readable report for the scope (SPEC §5.4).
func Stats(db *sql.DB, project, lang string) error {
	s, err := computeStats(db, project, lang)
	if err != nil {
		return err
	}

	pct := "n/a"
	if s.maxCombos > 0 {
		pct = fmt.Sprintf("%.2f%%", float64(s.remaining)/float64(s.maxCombos)*100)
	}

	fmt.Printf("Scope:        project=%s  language=%s\n", project, lang)
	fmt.Printf("Pool size:    %s tokens\n", formatInt(s.pool))
	fmt.Printf("Max combos:   %s  (n x (n-1))\n", formatInt(s.maxCombos))
	fmt.Printf("Used:         %s\n", formatInt(s.used))
	fmt.Printf("Remaining:    %s  (%s)\n", formatInt(s.remaining), pct)
	return nil
}
