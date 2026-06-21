---
name: random-names
description: >-
  Generate unique, anonymized full names (fake person names) for seed data, test
  fixtures, demos, or placeholder identities using the `random_name` CLI. Use
  whenever the user needs random / fake / anonymized people names — especially
  when each name must be unique per project or per language. Covers one-time
  setup, generating one or many names, capturing into shell variables, stats,
  and resetting a scope.
---

# random-names

`random_name` is a small Go CLI that draws two random tokens from a local SQLite
pool and joins them into a full name (e.g. `Cilvania Adrizia`). Names are kept
**unique per `(project, language)` scope** — no sequential suffixes, so output
stays anonymous. Full spec: `SPEC.md` in the tool's repo.

Use this skill when the user asks for fake/random/anonymized person names, test
users, seed data, or placeholder identities.

## Mental model: scopes

Uniqueness is tracked per **project slug** + **language**:

- `--project` / `-p` (default `default`) — names never repeat *within* a project.
- `--lang` / `-l` (default `pt-br`) — also picks which token pool to draw from.

Two different projects can both produce `João Silva`; the same project never
will. Reuse a project slug across runs to keep a coherent, collision-free set.

## One-time setup

Do this once per machine. Skip any step already satisfied.

### 1. Build and install on PATH

From the tool's repo:

```bash
CGO_ENABLED=0 go install .
# installs `random_name` into $GOBIN (or $GOPATH/bin). Ensure that dir is on PATH.
```

Verify: `command -v random_name`.

### 2. Create a central, reusable pool

The tool always reads/writes `random_names.db` **in the current working
directory**. To avoid re-bootstrapping in every project, keep one shared database
in `~/.random-names/` and always run the tool from there.

```bash
mkdir -p ~/.random-names
( cd ~/.random-names && random_name --bootstrap -f /path/to/repo/resources/pt-br.txt -l pt-br )
# optional: other languages
( cd ~/.random-names && random_name --bootstrap -f /path/to/repo/resources/en.txt -l en )
```

`resources/pt-br.txt` and `resources/en.txt` ship in the tool's repo. Bootstrap
is idempotent — re-running with the same (or an extended) file only adds new
tokens, so it's safe to repeat.

### 3. (Optional) convenience wrapper

Add to `~/.bashrc` so the central pool is used automatically from anywhere:

```bash
rn() { ( cd ~/.random-names && random_name "$@" ); }
```

Then just `rn -p myproject`.

## Usage

> When driving the tool yourself, always run it from the pool directory:
> `( cd ~/.random-names && random_name ... )`. The CWD is what selects the
> database, so running elsewhere would target an empty/new DB.

### Generate one name

```bash
( cd ~/.random-names && random_name -p myproject -l pt-br )
```

Only the name is printed to stdout, so it captures cleanly:

```bash
NAME=$( cd ~/.random-names && random_name -p myproject )
```

### Generate many unique names

Each call yields one name; loop for N:

```bash
( cd ~/.random-names && for i in $(seq 1 25); do random_name -p myproject -l pt-br; done )
```

All 25 are guaranteed distinct within `myproject` (until the pool is exhausted).

### Inspect a scope

```bash
( cd ~/.random-names && random_name --stats -p myproject -l pt-br )   # readable report
( cd ~/.random-names && random_name --count -p myproject -l pt-br )   # just the number left
```

### Reset a scope

Clears the used combinations for a `(project, language)` so they can be drawn
again. Destructive — prompts for confirmation unless `--force`:

```bash
( cd ~/.random-names && random_name --reset -p myproject -l pt-br --force )
```

## Flags

| Flag              | Default   | Meaning                                          |
|-------------------|-----------|--------------------------------------------------|
| `-p`, `--project` | `default` | Scope slug; uniqueness is per project.           |
| `-l`, `--lang`    | `pt-br`   | Language filter (generate) / tag (bootstrap).    |
| `-f`, `--file`    | —         | Source file for `--bootstrap`.                   |
| `--bootstrap`     | `false`   | Ingest names from `--file`.                      |
| `--stats`         | `false`   | Print a scope report.                            |
| `--count`         | `false`   | Print only remaining-combinations count.         |
| `--reset`         | `false`   | Delete the scope's recorded combinations.        |
| `-y`, `--force`   | `false`   | Skip the `--reset` confirmation.                 |

`--bootstrap`, `--reset`, `--stats`, `--count` are mutually exclusive; default
mode is generation.

## Exit codes & gotchas

- Exit `0` success · `1` runtime failure (pool `< 2` tokens, scope exhausted,
  DB/IO) · `2` usage error (bad flag, missing `--file`, conflicting modes).
- **DB location is the CWD** — if you forget to `cd ~/.random-names`, the tool
  creates a fresh empty DB in the current folder and generation fails with
  "run --bootstrap first". Always run from the pool dir.
- **Scope exhaustion**: a pool of `n` tokens yields at most `n × (n−1)` unique
  combinations per scope. On exhaustion the tool exits `1` with a clear message;
  either bootstrap more names or `--reset` the scope.
- If a requested language has no pool, bootstrap it first (step 2 above).
