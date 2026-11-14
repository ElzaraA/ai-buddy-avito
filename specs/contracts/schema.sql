-- СТАТУС: целевая схема хранения (roadmap). В прототипе данные лежат в памяти (internal/store)
-- и сохраняются снимком в JSON (STATE_FILE). Роли, статусы, форматы id и поля ниже повторяют
-- реальную модель прототипа: internal/auth/auth.go (роли), internal/store/store.go (действия,
-- история, шаги), internal/store/gaps.go (пробелы базы), internal/store/messages.go (диалоги),
-- specs/contracts/openapi.yaml (Action, OnboardingStep). Переход на PostgreSQL = новая
-- реализация Store с теми же методами; API и фронтенд не меняются.
-- Данные сотрудников и HR в Авито уже есть: здесь только синтетические копии для прототипа.

CREATE TABLE teams (
    id          TEXT PRIMARY KEY,               -- backend, frontend
    title       TEXT NOT NULL
);

CREATE TABLE users (
    id          TEXT PRIMARY KEY,               -- intern-1
    login       TEXT UNIQUE NOT NULL,           -- intern.backend (X-Demo-User; в Авито — логин SSO)
    name        TEXT NOT NULL,
    role        TEXT NOT NULL CHECK (role IN ('intern','buddy','lead','hr_bp')),
    team_id     TEXT NOT NULL REFERENCES teams(id)
);

-- Шаги чек-листа адаптации конкретного стажёра (store.ChecklistItem)
CREATE TABLE onboarding_steps (
    id          TEXT PRIMARY KEY,               -- c1-intern-1
    user_id     TEXT NOT NULL REFERENCES users(id),
    title       TEXT NOT NULL,
    stage       TEXT NOT NULL,                  -- week1, week2
    status      TEXT NOT NULL DEFAULT 'todo'
                CHECK (status IN ('todo','waiting','done')),
    kind        TEXT CHECK (kind IN ('access')),-- шаг закрывается согласованной заявкой на доступ
    related_action_id TEXT,                     -- REQ-000001: заявка, которой ждёт шаг
    owner       TEXT NOT NULL,                  -- кто отвечает за шаг (стажёр, наставник)
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Действия, подготовленные агентом черновиком (store.Absence; имя типа историческое).
-- «Выполнено» = результат сохранён здесь и показан пользователю.
CREATE TABLE actions (
    id          TEXT PRIMARY KEY,               -- ABS-000001 (отгул), REQ-000001 (доступ)
    type        TEXT NOT NULL CHECK (type IN ('absence','access')),
    user_id     TEXT NOT NULL REFERENCES users(id),
    team_id     TEXT REFERENCES teams(id),      -- руководитель согласует только заявки своей команды
    resource    TEXT,                           -- для access: gitlab, jira, grafana…
    date_from   DATE,                           -- для absence
    date_to     DATE,
    reason      TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL
                CHECK (status IN ('draft','pending','approved','rejected','cancelled')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ
);

-- История действия: кто и когда перевёл его в статус (store.Event, поле history в API)
CREATE TABLE action_history (
    action_id   TEXT NOT NULL REFERENCES actions(id),
    at          TIMESTAMPTZ NOT NULL,
    by          TEXT NOT NULL REFERENCES users(id),
    status      TEXT NOT NULL
                CHECK (status IN ('draft','pending','approved','rejected','cancelled')),
    comment     TEXT
);

-- Вопросы без ответа в базе знаний (store.Gap). Без user_id: наставник видит, что дописать, но не кто спросил.
CREATE TABLE knowledge_gaps (
    team_id     TEXT NOT NULL REFERENCES teams(id),
    question_key TEXT NOT NULL,                 -- нормализованный вопрос (нижний регистр, без ?!.)
    question    TEXT NOT NULL,                  -- до 200 символов
    count       INT  NOT NULL DEFAULT 1,
    last_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (team_id, question_key)
);

-- Асинхронный чат (store.Session, store.Message)
CREATE TABLE chat_sessions (
    id          UUID PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE chat_messages (
    id          UUID PRIMARY KEY,
    session_id  UUID NOT NULL REFERENCES chat_sessions(id),
    role        TEXT NOT NULL CHECK (role IN ('user','assistant')),
    content     TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('pending','in_progress','done','failed')),
    result      JSONB,                          -- ответ агента: type, text, sources, args
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- База знаний в прототипе — markdown в KNOWLEDGE_DIR с разметкой <!-- stage: team: audience: -->;
-- в Авито — выгрузка из Confluence. Таблица нужна, только если хранить фрагменты в СУБД.
CREATE TABLE kb_chunks (
    id          TEXT PRIMARY KEY,
    source      TEXT NOT NULL,                  -- файл и раздел, показывается как источник ответа
    team_id     TEXT REFERENCES teams(id),      -- NULL = общий фрагмент
    audience    TEXT[] NOT NULL,                -- роли; пусто = не виден никому (deny-by-default)
    body        TEXT NOT NULL,
    is_test_data BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX ON actions (user_id, created_at DESC);
CREATE INDEX ON actions (team_id, status);
CREATE INDEX ON action_history (action_id, at);
CREATE INDEX ON chat_messages (session_id, created_at);
