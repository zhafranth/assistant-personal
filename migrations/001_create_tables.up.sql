CREATE TABLE projects (
    id          SERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    due_date    TIMESTAMPTZ,
    is_active   BOOLEAN DEFAULT TRUE,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE todos (
    id            SERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL,
    project_id    INT REFERENCES projects(id) ON DELETE CASCADE,
    title         TEXT NOT NULL,
    description   TEXT,
    is_completed  BOOLEAN DEFAULT FALSE,
    completed_at  TIMESTAMPTZ,
    due_date      TIMESTAMPTZ,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE reminders (
    id               SERIAL PRIMARY KEY,
    todo_id          INT NOT NULL REFERENCES todos(id) ON DELETE CASCADE,
    remind_at        TIMESTAMPTZ NOT NULL,
    is_recurring     BOOLEAN DEFAULT FALSE,
    recurrence_rule  TEXT,
    last_fired_at    TIMESTAMPTZ,
    is_active        BOOLEAN DEFAULT TRUE,
    created_at       TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_reminders_active ON reminders (remind_at)
    WHERE is_active = TRUE;

CREATE TABLE expenses (
    id            SERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL,
    description   TEXT NOT NULL,
    amount        BIGINT NOT NULL,
    recorded_at   TIMESTAMPTZ DEFAULT NOW()
);
