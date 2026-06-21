# random_name — Technical Specification

## 1. Overview

`random_name` is a small, high-performance CLI tool written in Go. It generates
unique, anonymized full names by drawing two distinct tokens at random from a
single name pool stored in a local SQLite database and joining them
(`"Token1 Token2"`).

Generation is **scoped by project** and **filtered by language**. Uniqueness is
guaranteed per `(project, language)` scope: a combination that was already
produced for that scope is never emitted again. Collisions are resolved by an
automatic retry mechanism — there are no sequential suffixes, so output stays
naturally anonymous.

### Design notes

- There is **no separate "first name" / "last name" pool**. Both tokens come
  from the same `source_names` table; any token may appear in either position.
  `"Vader Xerxes"` and `"Xerxes Vader"` are two valid, distinct results.
- The tool is built to be embedded in shell scripts and pipelines. On success it
  prints **only** the generated name to `stdout`.

---

## 2. Non-Functional Requirements

- **Zero-overhead output.** A successful generation prints the name and nothing
  else to `stdout`, so it can be captured directly:
  `NAME=$(./random_name)`. All diagnostics go to `stderr`.
- **Standalone binary.** Compiles to a single, statically linked native binary
  with no runtime dependencies.
- **No CGO.** Use a pure-Go SQLite driver (`modernc.org/sqlite`) so the build
  requires neither CGO nor a C toolchain on the host.
- **Self-contained storage.** The database lives in a single file in the current
  working directory; no external services or path resolution outside the CWD.

---

## 3. Database

### 3.1 File location

The database file is named `random_names.db` and is always opened (or created)
in the **current working directory** where the command runs. No other path
resolution is performed.

On every run, the tool opens the database with:

```sql
PRAGMA journal_mode = WAL;     -- safe concurrent readers/writers
PRAGMA busy_timeout = 5000;    -- wait up to 5s instead of failing on a lock
```

WAL plus `busy_timeout` makes it safe to run several instances concurrently
against the same file (see §5.3).

### 3.2 Schema

```sql
-- Raw pool of available name tokens.
CREATE TABLE IF NOT EXISTS source_names (
    id       INTEGER PRIMARY KEY,   -- rowid alias; no AUTOINCREMENT needed
    name     TEXT NOT NULL,
    language TEXT NOT NULL,
    UNIQUE (language, name)
);

-- Speeds up the WHERE language = ? filter when multiple languages coexist.
CREATE INDEX IF NOT EXISTS idx_source_names_lang
    ON source_names (language);

-- Tracks combinations already produced, per scope.
CREATE TABLE IF NOT EXISTS utilized_names (
    project_slug TEXT NOT NULL,
    language     TEXT NOT NULL,
    full_name    TEXT NOT NULL,
    PRIMARY KEY (project_slug, language, full_name)
);
```

Notes:

- `source_names.id` is a plain `INTEGER PRIMARY KEY` (rowid alias). `AUTOINCREMENT`
  is intentionally **not** used — it adds a `sqlite_sequence` table and overhead
  with no benefit here.
- The `idx_source_names_lang` index accelerates the `WHERE language = ?` filter.
  It does **not** speed up `ORDER BY random()` (that still scans + sorts the
  filtered rows); for a dictionary of this size that cost is negligible.
- `utilized_names` uses a composite primary key as the uniqueness guard for the
  collision-detection strategy in §5.2.

---

## 4. CLI Interface

The binary runs in one of several modes. The active mode is selected by the
first mode flag present, in this precedence order:

1. `--bootstrap` (§4.2)
2. `--reset` (§4.4)
3. `--stats` (§4.3)
4. `--count` (§4.3)
5. otherwise → **generation** (§4.1, default)

Mode flags are mutually exclusive; combining two of them is a usage error
(exit code 2). All modes accept `--project`/`-p` and `--lang`/`-l` with the same
defaults (`"default"` / `"pt-br"`).

### 4.1 Generation mode (default)

| Flag             | Type   | Default     | Description                     |
|------------------|--------|-------------|---------------------------------|
| `--project`, `-p`| string | `"default"` | Project slug that scopes uniqueness. |
| `--lang`, `-l`   | string | `"pt-br"`   | Language filter for the token pool.  |

```bash
./random_name
./random_name --project ecommerce_prod --lang en
./random_name -p ecommerce_prod -l en
```

### 4.2 Bootstrap mode (data ingestion)

| Flag             | Type   | Default | Description                                  |
|------------------|--------|---------|----------------------------------------------|
| `--bootstrap`    | bool   | `false` | Switches to ingestion mode.                  |
| `--file`, `-f`   | string | —       | Path to a UTF-8 text file, one name per line.|
| `--lang`, `-l`   | string | `"pt-br"`| Language code tagged onto every ingested name.|

```bash
./random_name --bootstrap --file resources/pt-br.txt --lang pt-br
./random_name --bootstrap -f resources/en.txt -l en
```

`--file` is required in bootstrap mode; its absence is a usage error (exit
code 2).

### 4.3 Stats modes (read-only)

Both inspect a scope without modifying anything.

| Flag        | Type | Description                                                        |
|-------------|------|-------------------------------------------------------------------|
| `--count`   | bool | Print **only** the number of remaining combinations for the scope.|
| `--stats`   | bool | Print a human-readable report for the scope.                       |

`--count` is script-friendly (single number on `stdout`); `--stats` is for
humans. Both honor `--project` / `--lang`.

```bash
REMAINING=$(./random_name --count -p ecommerce_prod -l pt-br)
./random_name --stats -p ecommerce_prod -l pt-br
```

### 4.4 Reset mode (destructive)

Clears the recorded combinations for a scope so they can be generated again.

| Flag             | Type | Description                                                 |
|------------------|------|-------------------------------------------------------------|
| `--reset`        | bool | Delete the scope's rows from `utilized_names`.              |
| `--force`, `-y`  | bool | Skip the interactive confirmation prompt (for scripts).    |

Scope is `(project, language)` — the same scope used by generation. It does
**not** touch other projects or other languages.

```bash
./random_name --reset -p ecommerce_prod -l pt-br          # prompts y/N
./random_name --reset -p ecommerce_prod -l pt-br --force  # no prompt
```

---

## 5. Workflows

### 5.1 Bootstrap (ingestion)

1. Open/create `random_names.db` in the CWD; apply pragmas; ensure schema (§3.2).
2. Validate flags: `--file` must be provided and readable.
3. Read the file **line by line** (it is treated as plain text, *not* CSV —
   no delimiter or quote parsing). For each line:
   - Strip a leading UTF-8 BOM on the first line.
   - Trim surrounding whitespace and line endings (`\r`, `\n`).
   - **Skip empty lines.**
   - **Normalize case** to Title Case for a single token: first rune uppercase,
     remaining runes lowercase, Unicode-aware. Examples:
     `MARCELO → Marcelo`, `ARAÚJO → Araújo`, `ASSUNÇÃO → Assunção`.
4. Insert all rows inside a single transaction
   (`BEGIN … COMMIT`) for speed:

   ```sql
   INSERT OR IGNORE INTO source_names (name, language) VALUES (?, ?);
   ```

   `INSERT OR IGNORE` makes ingestion idempotent and cumulative — re-running the
   same file, or overlapping files, adds only the new tokens without error.
5. Print a summary to `stdout` (e.g. lines read, inserted, skipped/duplicates)
   and exit `0`.

> Case normalization is what keeps the `UNIQUE (language, name)` constraint
> meaningful: without it, `MARCELO` and `Marcelo` would be stored as two
> distinct tokens.

### 5.2 Generation

1. Open `random_names.db` in the CWD; apply pragmas; ensure schema.
2. **Pool sanity check** — count tokens for the language:

   ```sql
   SELECT COUNT(*) FROM source_names WHERE language = ?;
   ```

   If the count is `< 2`, abort with a clear error to `stderr` telling the user
   to bootstrap the language first, and exit `1`.
3. **Retry loop** (counter starts at 0; max 100 attempts):
   - **Failsafe:** if the counter reaches `100`, abort. Print to `stderr` an
     error that distinguishes the two likely causes, ideally using the known
     pool size `n` and the count of used combinations for the scope:
     - pool exhausted for the scope (used combinations ≈ the theoretical
       maximum `n × (n − 1)`), or
     - pool too small / persistent bad luck.

     Exit `1`.
   - Draw two distinct tokens:

     ```sql
     SELECT name FROM source_names WHERE language = ? ORDER BY random() LIMIT 2;
     ```

     (Two distinct rows ⇒ two distinct names, guaranteed by the unique
     constraint.)
   - Build the candidate: `full_name = "{Name1} {Name2}"`.
   - Attempt to claim it for the scope:

     ```sql
     INSERT OR IGNORE INTO utilized_names (project_slug, language, full_name)
     VALUES (?, ?, ?);
     ```

   - **Collision handling via `RowsAffected`** (no error-string parsing):
     - `RowsAffected == 1` → newly claimed. Print `full_name` to `stdout` and
       exit `0`.
     - `RowsAffected == 0` → already used for this scope. Increment the counter
       and retry.

> Using `INSERT OR IGNORE` + `RowsAffected()` instead of catching a primary-key
> violation keeps collision detection robust and driver-independent.

### 5.3 Concurrency

Concurrent invocations for the same scope are safe by construction: two
processes may draw the same candidate, but only one `INSERT` claims it
(`RowsAffected == 1`); the other sees `RowsAffected == 0` and retries. WAL mode
and `busy_timeout` (§3.1) prevent `database is locked` failures under contention.

### 5.4 Stats (`--stats` / `--count`)

1. Open the DB; apply pragmas; ensure schema.
2. Compute, for the scope:
   - `n` = pool size: `SELECT COUNT(*) FROM source_names WHERE language = ?`
   - `max` = theoretical maximum combinations = `n × (n − 1)`
   - `used` = `SELECT COUNT(*) FROM utilized_names WHERE project_slug = ? AND language = ?`
   - `remaining` = `max − used`
3. Output:
   - `--count` → print just `remaining` to `stdout` and exit `0`.
   - `--stats` → print a readable block to `stdout` and exit `0`, e.g.:

     ```
     Scope:        project=ecommerce_prod  language=pt-br
     Pool size:    103,420 tokens
     Max combos:   10,695,667,980  (n x (n-1))
     Used:         1,245
     Remaining:    10,695,666,735  (99.99%)
     ```

These modes never write to the database.

### 5.5 Reset (`--reset`)

1. Open the DB; apply pragmas; ensure schema.
2. Count what would be deleted:
   `SELECT COUNT(*) FROM utilized_names WHERE project_slug = ? AND language = ?`.
3. Confirmation:
   - Without `--force`: prompt on `stderr`
     (e.g. `Delete N combinations for scope project=… language=…? [y/N]`).
     Proceed only on an explicit `y` / `yes`; otherwise abort with exit `0` and a
     "cancelled" notice on `stderr`.
   - With `--force`: skip the prompt.
4. Delete the scope's rows:
   `DELETE FROM utilized_names WHERE project_slug = ? AND language = ?`.
5. Print the number of deleted combinations to `stdout` and exit `0`.

---

## 6. Error Handling & Exit Codes

| Code | Meaning                                                             |
|------|--------------------------------------------------------------------|
| `0`  | Success (name generated, or bootstrap completed).                  |
| `1`  | Runtime failure (pool `< 2` tokens, max retries reached, DB/IO error). |
| `2`  | Usage error (unknown flag, missing required `--file` in bootstrap).|

All human-readable messages go to `stderr`. In generation mode, `stdout` carries
the generated name and nothing else.

---

## 7. Implementation Notes

- **Language / module:** Go. Suggested module path
  `github.com/marceloxp/random-names`, binary `random_name`.
- **Dependencies:** `modernc.org/sqlite` (pure Go). For Unicode-aware Title
  casing of a single token, the standard `unicode` + `strings` packages are
  sufficient (uppercase the first rune, lowercase the rest); `strings.Title` is
  deprecated and should be avoided.
- **Suggested layout:**

  ```
  random-names/
  ├── main.go            # flag parsing, mode dispatch
  ├── internal/db/       # open, pragmas, schema, queries
  ├── internal/generate/ # generation retry loop
  ├── internal/bootstrap/# ingestion + normalization
  ├── resources/         # seed name files (pt-br.txt, …)
  └── SPEC.md
  ```

- **Build:** `CGO_ENABLED=0 go build -o random_name .`

---

## 8. Future Considerations (out of scope for v1)

- Smarter random sampling if very large pools ever make `ORDER BY random()` a
  bottleneck — e.g. drawing by random `rowid` window instead of a full scan +
  sort. Not worth it at the current pool size (~100k); kept simple in v1.
- A `--json` output option for `--stats`.
