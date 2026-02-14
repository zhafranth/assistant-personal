# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build ./...

# Run (requires .env)
go run ./cmd/bot/main.go

# Run with live DB (docker)
docker-compose up -d   # starts PostgreSQL
go run ./cmd/bot/main.go
```

Required `.env` variables: `TELEGRAM_BOT_TOKEN`, `DATABASE_URL`, `ANTHROPIC_API_KEY`, `TIMEZONE` (default: Asia/Jakarta), `SCHEDULER_INTERVAL_SEC` (default: 30).

## Architecture

```
cmd/bot/main.go              Entry point — wires everything, starts goroutines
internal/
  config/                    Loads env vars
  db/postgres.go             DB connect + golang-migrate runner (file:///migrations)
  nlp/
    service.go               Claude Haiku caller — returns []ParsedIntent (always array)
    types.go                 ParsedIntent struct + date/time parse helpers
  bot/
    handler.go               Telegram handler — routes []ParsedIntent, calls services
    formatter.go             All display formatting (todo list, daily briefing, etc.)
  module/
    todo/                    Todo CRUD + soft delete
    expense/                 Expense tracking with paid/unpaid status
    project/                 Projects with goals (todos linked via project_id)
  reminder/
    repository.go            Reminder CRUD + ListActiveByUser + UpsertByTodoID
    scheduler.go             Ticker goroutine that fires due reminders
migrations/                  golang-migrate SQL files (NNN_name.up.sql / .down.sql)
```

## Key Patterns

**NLP always returns `[]ParsedIntent`** — the system prompt instructs Claude to return a JSON array. The handler loops through all intents and joins responses with `"\n\n"`. Never expect a single object.

**Bulk operations** work automatically — NLP splits "tambah todo A, B, C" into 3 `add_todo` intents; the handler loop handles the rest.

**Soft delete on todos** — `deleted_at TIMESTAMPTZ` column. All queries include `AND deleted_at IS NULL`. An hourly goroutine in `main.go` calls `todoSvc.CleanupCompletedTodos()` to soft-delete completed todos older than 24 hours.

**`clear_todo` = hard DELETE** — removes all todos permanently (different from `complete_todo` which marks done, and different from soft-delete cleanup). Only triggers when user says "kosongkan todo" without naming specific items.

**Reminder upsert** — `reminder.Repository.UpsertByTodoID` updates existing active reminder or creates a new non-recurring one. Used by `todo.Service.Edit` when `remind_at` is provided.

**Expense disambiguation** — when multiple expenses share the same name, `pickExpense` filters by `amount` and/or `date`. If still ambiguous, returns a disambiguation message asking user to be more specific.

**All user-facing text is in Indonesian.**

## Module Responsibilities

| Module | Service methods |
|--------|----------------|
| `todo` | Add, List, Complete, Edit, Delete, ClearAll, CleanupCompletedTodos |
| `expense` | Add, List, PayExpense, Edit, Delete, ClearByMonth, MonthlyReport |
| `project` | Add, AddGoal, CompleteGoal, List, Show, Delete, DeleteGoal |
| `reminder` | Scheduler fires due reminders; recurring rules: `daily`, `weekly:MON`, `monthly:5`, `yearly:3-15` |

## NLP Intent Reference

Full intent list lives in `internal/nlp/service.go` system prompt. Critical disambiguation rules in that prompt:
- `clear_todo` — only when NO specific todo names mentioned
- `pay_expense` — for "lunasi X", never `add_expense`
- `edit_expense` — for renaming or changing paid status
- `clear_expense` — bulk delete by month/year
