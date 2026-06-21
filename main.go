package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/marceloxp/random-names/internal/core"
)

const usage = `random_name - generate unique, anonymized full names from a local SQLite pool.

Modes (mutually exclusive):
  (default)      Generate one unique "Token1 Token2" name for the scope.
  --bootstrap    Ingest names from a text file (one name per line).
  --stats        Print a human-readable report for the scope.
  --count        Print only the number of remaining combinations for the scope.
  --reset        Delete the scope's recorded combinations.

Flags:
  -p, --project string   Project slug that scopes uniqueness (default "default")
  -l, --lang    string   Language code filter / tag (default "pt-br")
  -f, --file    string   Source file for --bootstrap (one name per line)
  -y, --force            Skip the --reset confirmation prompt

Examples:
  random_name -p ecommerce_prod -l en
  random_name --bootstrap --file resources/pt-br.txt --lang pt-br
  random_name --stats -p ecommerce_prod
  REMAINING=$(random_name --count -p ecommerce_prod)
  random_name --reset -p ecommerce_prod --force
`

// exit codes (SPEC §6)
const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2
)

func main() {
	var (
		project, lang, file            string
		bootstrap, stats, count, reset bool
		force                          bool
	)

	fs := flag.NewFlagSet("random_name", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	// Long names and short aliases share the same target variable.
	fs.StringVar(&project, "project", "default", "project slug")
	fs.StringVar(&project, "p", "default", "project slug (shorthand)")
	fs.StringVar(&lang, "lang", "pt-br", "language code")
	fs.StringVar(&lang, "l", "pt-br", "language code (shorthand)")
	fs.StringVar(&file, "file", "", "source file for --bootstrap")
	fs.StringVar(&file, "f", "", "source file (shorthand)")
	fs.BoolVar(&bootstrap, "bootstrap", false, "ingest names from --file")
	fs.BoolVar(&stats, "stats", false, "print a stats report")
	fs.BoolVar(&count, "count", false, "print remaining combinations count")
	fs.BoolVar(&reset, "reset", false, "clear the scope's used combinations")
	fs.BoolVar(&force, "force", false, "skip --reset confirmation")
	fs.BoolVar(&force, "y", false, "skip --reset confirmation (shorthand)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		// flag already printed the usage block.
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(exitOK) // -h / --help is not an error
		}
		os.Exit(exitUsage)
	}

	// Modes are mutually exclusive (SPEC §4).
	if nModes := countTrue(bootstrap, reset, stats, count); nModes > 1 {
		fmt.Fprintln(os.Stderr, "Error: --bootstrap, --reset, --stats and --count are mutually exclusive.")
		os.Exit(exitUsage)
	}

	if err := run(project, lang, file, bootstrap, reset, stats, count, force); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(exitRuntime)
	}
	os.Exit(exitOK)
}

func run(project, lang, file string, bootstrap, reset, stats, count, force bool) error {
	// Bootstrap needs --file before we even touch the database.
	if bootstrap && file == "" {
		fmt.Fprintln(os.Stderr, "Error: --bootstrap requires --file.")
		os.Exit(exitUsage)
	}

	db, err := core.Open()
	if err != nil {
		return err
	}
	defer db.Close()

	// Dispatch in the precedence order from SPEC §4.
	switch {
	case bootstrap:
		return core.Bootstrap(db, file, lang)
	case reset:
		return core.Reset(db, project, lang, force)
	case stats:
		return core.Stats(db, project, lang)
	case count:
		return core.Count(db, project, lang)
	default:
		return core.Generate(db, project, lang)
	}
}

func countTrue(flags ...bool) int {
	n := 0
	for _, f := range flags {
		if f {
			n++
		}
	}
	return n
}
