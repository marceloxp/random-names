package core

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"
)

// Reset clears the recorded combinations for the (project, lang) scope.
// Unless force is true, it asks for interactive confirmation on stderr first.
// See SPEC §5.5.
func Reset(db *sql.DB, project, lang string, force bool) error {
	pending, err := usedCount(db, project, lang)
	if err != nil {
		return fmt.Errorf("counting combinations: %w", err)
	}

	if pending == 0 {
		fmt.Printf("Nothing to reset for project=%s language=%s\n", project, lang)
		return nil
	}

	if !force {
		fmt.Fprintf(os.Stderr,
			"Delete %s combinations for scope project=%s language=%s? [y/N] ",
			formatInt(pending), project, lang)

		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			fmt.Fprintln(os.Stderr, "Cancelled.")
			return nil
		}
	}

	res, err := db.Exec(
		`DELETE FROM utilized_names WHERE project_slug = ? AND language = ?`,
		project, lang)
	if err != nil {
		return fmt.Errorf("deleting combinations: %w", err)
	}

	deleted, _ := res.RowsAffected()
	fmt.Printf("Reset complete: %s combinations deleted for project=%s language=%s\n",
		formatInt(deleted), project, lang)
	return nil
}
