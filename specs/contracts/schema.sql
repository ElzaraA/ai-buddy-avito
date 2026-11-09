-- Модель данных AI-buddy (PostgreSQL). Владелец — backend.
-- Данные сотрудников/HR в Авито уже есть: здесь только синтетические копии для прототипа.

CREATE TABLE teams (
    id          TEXT PRIMARY KEY,               -- team_payments
    title       TEXT NOT NULL
);

CREATE TABLE users (
    id          TEXT PRIMARY KEY,               -- u_intern_1
    login       TEXT UNIQUE NOT NULL,           -- тестовый логин для жюри
    name        TEXT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('intern','mentor','team_lead','hrbp')),
    team_id     TEXT NOT NULL REFERENCES teams(id),
    mentor_id   TEXT REFERENCES users(id),
    start_date  DATE
);

-- Шаблон адаптации (общий + командный)
CREATE TABLE onboarding_stages (
    id          TEXT PRIMARY KEY,               -- week_1
    title       TEXT NOT NULL,
    ord         INT  NOT NULL
);

CREATE TABLE onboarding_step_templates (
    id          TEXT PRIMARY KEY,               -- step_gitlab_access
    stage_id    TEXT NOT NULL REFERENCES onboarding_stages(id),
    team_id     TEXT REFERENCES teams(id),      -- NULL = для всех команд
    title       TEXT NOT NULL,
    description TEXT,
    due_offset_days INT                         -- срок от start_date
);

-- Прогресс конкретного стажёра
CREATE TABLE onboarding_steps (
    user_id     TEXT NOT NULL REFERENCES users(id),
    step_id     TEXT NOT NULL REFERENCES onboarding_step_templates(id),
    status      TEXT NOT NULL DEFAULT 'todo'
                CHECK (status IN ('todo','in_progress','waiting','done','blocked')),
    related_action_id UUID,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, step_id)
);

CREATE TABLE chat_sessions (
    id          UUID PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE chat_messages (
    id          UUID PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES chat_sessions(id),
    role        TEXT NOT NULL CHECK (role IN ('user','assistant')),
    text        TEXT NOT NULL,
    sources     JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Действия через корпоративные сервисы ("выполнено" = результат вернулся и сохранён здесь)
CREATE TABLE actions (
    id          UUID PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    message_id  UUID REFERENCES chat_messages(id),
    service     TEXT NOT NULL CHECK (service IN ('absence','access','checklist')),
    type        TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('pending','succeeded','failed')),
    external_id TEXT,
    summary     TEXT,
    payload     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- База знаний (тестовые инструкции, НЕ регламенты Авито)
CREATE TABLE kb_documents (
    id          TEXT PRIMARY KEY,
    team_id     TEXT REFERENCES teams(id),      -- NULL = общий документ
    visibility  TEXT NOT NULL CHECK (visibility IN ('public','team','mentors_only')),
    title       TEXT NOT NULL,
    body        TEXT NOT NULL,
    is_test_data BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX ON actions (user_id, created_at DESC);
CREATE INDEX ON chat_messages (session_id, created_at);
