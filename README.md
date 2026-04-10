<picture>
  <source media="(prefers-color-scheme: dark)" srcset="logo-dark.svg">
  <img src="logo.svg" alt="kronk" width="420">
</picture>

**Pull the lever.**

<img src="https://media1.tenor.com/m/mcS-PaTlDawAAAAd/pull-the-lever-wrong-lever.gif" width="360">

[![CI](https://github.com/janpgu/kronk/actions/workflows/ci.yml/badge.svg)](https://github.com/janpgu/kronk/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/go-1.22+-00ADD8?style=flat&logo=go&logoColor=white)
![License](https://img.shields.io/badge/license-MIT-green?style=flat)
![Platform](https://img.shields.io/badge/platform-windows%20%7C%20macos%20%7C%20linux-lightgrey?style=flat)

---

kronk is a job scheduler that runs on a single SQLite file. No Redis, no message broker, no always-on daemon. You add one line to crontab and kronk takes it from there: check what's due, run it, record the result, exit. kronk will pull the lever for you. The right one, this time. Probably.

It's for people who need scheduled tasks on a machine they control, like a home server, a VPS, a dev box, and don't want to manage infrastructure to do it. If you've ever added five cron jobs and realized you have no idea which ones ran, what they output, or why one failed three weeks ago, kronk solves that.

---

## Install

**Linux / macOS** (one line):

```sh
curl -fsSL https://raw.githubusercontent.com/janpgu/kronk/main/install.sh | sh
```

Installs the binary to `/usr/local/bin` and adds the crontab entry automatically.

**Windows** (one line in PowerShell):

```powershell
irm https://raw.githubusercontent.com/janpgu/kronk/main/install.ps1 | iex
```

Installs the binary to `~/bin`, adds it to PATH, and registers a Task Scheduler task that runs every minute, including on battery.

**Or build from source:**

```sh
go install github.com/janpgu/kronk@latest
```

Then run `kronk doctor` to complete setup.

Requires Go 1.22+. No C compiler needed as the SQLite driver is pure Go.

---

## Quick start

```sh
# Run a Python script every night at 2am
kronk add "python backup.py" --name backup --schedule "every night"

# Sync something every 5 minutes
kronk add "python sync.py" --name sync --schedule "every 5 minutes"

# See what's scheduled
kronk status

# See what ran and whether it succeeded
kronk history

# Run a job right now without waiting for the schedule
kronk trigger backup
```

---

## Commands

| Command | Description |
|---|---|
| `kronk add <cmd> --name <n> --schedule <s>` | Add a new job |
| `kronk status` | Show all jobs and their next run time |
| `kronk show <name>` | Show all details for a single job |
| `kronk pause <name>` | Pause a job without removing it |
| `kronk resume <name>` | Resume a paused job |
| `kronk history [--job <n>] [--limit 20]` | Show recent run history |
| `kronk tick [--verbose]` | Run all due jobs (called by crontab) |
| `kronk run` | Long-running mode: tick every 30 seconds |
| `kronk trigger <name>` | Run a job immediately |
| `kronk remove <name>` | Remove a job and its history |
| `kronk prune [--days N]` | Delete run history older than N days (default 30) |
| `kronk edit` | Edit all jobs in $EDITOR |
| `kronk doctor` | Print config and setup instructions |
| `kronk version` | Print the kronk version |

**Flags available on all commands:**

| Flag | Description |
|---|---|
| `--db <path>` | Use a specific database file |

The database path resolves in this order: `--db` flag → `KRONK_DB` environment variable → platform default (`~/.kronk/kronk.db` on Unix, `%APPDATA%\kronk\kronk.db` on Windows).

---

## Schedules

Natural language or raw cron, both work:

| Input | Cron |
|---|---|
| `every minute` | `* * * * *` |
| `every 5 minutes` | `*/5 * * * *` |
| `every hour` | `0 * * * *` |
| `every 6 hours` | `0 */6 * * *` |
| `every day at 9am` | `0 9 * * *` |
| `every day at 3:30pm` | `30 15 * * *` |
| `every night` | `0 2 * * *` |
| `every morning` | `0 7 * * *` |
| `every monday` | `0 9 * * 1` |
| `every monday at 9am` | `0 9 * * 1` |
| `every weekday` | `0 9 * * 1-5` |
| `every weekend` | `0 10 * * 6,0` |
| `twice a day` | `0 9,21 * * *` |
| `0 2 * * *` | passed through as-is |

---

## Retries

```sh
kronk add "python flaky.py" --name flaky --schedule "every hour" --retries 3
```

On failure, kronk retries with exponential backoff: 2s, 4s, 8s after each attempt. After all retries are exhausted the job is marked `failed` and won't run again until you reset it with `kronk edit`.

---

## How it works

The architecture is a tick model. The OS wakes kronk once per minute; kronk checks the database for jobs whose `next_run_at` has passed, runs them as subprocesses, records stdout/stderr and exit code, updates `next_run_at`, and exits. There is no long-running process maintaining state in memory.

This has a few consequences worth understanding:

- **Crash recovery is free.** If the machine reboots mid-run, the next tick will see the job's `finished_at` is null, recognize it as a stale running instance, and skip it. No orphaned workers, no stuck queues.
- **The scheduler has one-minute resolution by default.** If you need sub-minute jobs, run `kronk run` as a persistent process instead (ticks every 30 seconds).
- **Jobs are shell commands.** `kronk` runs them with `sh -c` on Unix and `cmd /C` on Windows. Any executable, script, or pipeline works. The job has no knowledge of kronk.
- **Concurrency is opt-out.** If a job is still running when the next tick fires (e.g. a job that takes 90 seconds on a one-minute schedule), the second tick skips it rather than starting a second instance.

All state (jobs, run history, stdout, stderr, exit codes) lives in a single SQLite file. You can inspect it directly with any SQLite client, back it up with `cp`, or move it to another machine by copying the file.

---

## Comparison

**vs. cron:** cron has no run history, no retry logic, no way to see what a job output last Tuesday. kronk adds those things and keeps the same mental model.

**vs. APScheduler / Celery:** those are libraries for Python applications. You embed them in your code and manage a worker process. kronk is an external tool that runs any command; your scripts don't need to know it exists.

**vs. a hosted scheduler (GitHub Actions cron, AWS EventBridge):** valid choices if you're already in those ecosystems. kronk is for when you just have a machine and want things to run on it.

**Why not just use cron?** Cron is fine. kronk is cron with a history log, retry handling, a status view, and a slightly more forgiving schedule syntax. If you don't need any of that, cron is the right tool.

---

## Contributing

Bug reports and pull requests are welcome. Open an issue first for anything beyond small fixes. Run `go vet ./...` and `go test ./...` before submitting.
