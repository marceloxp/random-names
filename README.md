# random-names

A small, fast CLI that generates unique, anonymized full names by combining two
random tokens from a local SQLite pool. Names are unique **per project and
language scope** — no sequential suffixes, so output stays anonymous.

Written in Go with a pure-Go SQLite driver (`modernc.org/sqlite`): no CGO, no C
toolchain, single static binary.

See [`SPEC.md`](SPEC.md) for the full specification.

## Build

```bash
CGO_ENABLED=0 go build -o random_name .
```

## Usage

The database `random_names.db` is created in the current working directory.

### 1. Load names (bootstrap)

Ingest a plain-text file, one name per line. Names are normalized to Title Case
(`MARCELO → Marcelo`) and de-duplicated; re-running is safe and cumulative.

```bash
./random_name --bootstrap --file resources/pt-br.txt --lang pt-br
```

### 2. Generate a name

```bash
./random_name                              # default project, pt-br
./random_name -p ecommerce_prod -l en      # scoped by project + language
NAME=$(./random_name -p ecommerce_prod)    # capture in a shell variable
```

On success only the name is printed to stdout, so it pipes cleanly.

### 3. Inspect a scope

```bash
./random_name --stats -p ecommerce_prod          # human-readable report
REMAINING=$(./random_name --count -p ecommerce_prod)   # just the number
```

### 4. Reset a scope

Clears the recorded combinations for a `(project, language)` scope so they can be
generated again.

```bash
./random_name --reset -p ecommerce_prod -l pt-br          # asks to confirm
./random_name --reset -p ecommerce_prod -l pt-br --force  # no prompt
```

## Flags

| Flag              | Default     | Description                                   |
|-------------------|-------------|-----------------------------------------------|
| `-p`, `--project` | `default`   | Project slug that scopes uniqueness.          |
| `-l`, `--lang`    | `pt-br`     | Language code (filter on generate, tag on bootstrap). |
| `-f`, `--file`    | —           | Source file for `--bootstrap`.                |
| `--bootstrap`     | `false`     | Ingest names from `--file`.                   |
| `--stats`         | `false`     | Print a report for the scope.                 |
| `--count`         | `false`     | Print only the remaining-combinations count.  |
| `--reset`         | `false`     | Delete the scope's recorded combinations.     |
| `-y`, `--force`   | `false`     | Skip the `--reset` confirmation prompt.       |

Modes (`--bootstrap`, `--reset`, `--stats`, `--count`) are mutually exclusive;
the default mode is generation.

## Exit codes

| Code | Meaning                                                      |
|------|-------------------------------------------------------------|
| `0`  | Success.                                                    |
| `1`  | Runtime failure (pool too small, scope exhausted, DB/IO).  |
| `2`  | Usage error (bad flag, missing `--file`, conflicting modes).|
