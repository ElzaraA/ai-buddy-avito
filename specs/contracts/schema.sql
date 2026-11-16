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
    sub_done    INT,                            -- kind=access: сколько систем (gitlab/git, jira, grafana) одобрено
    sub_total   INT,                            -- kind=access: 3; шаг done только при sub_done = sub_total
    missing     TEXT[],                         -- kind=access: системы без заявки
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

-- Сообщения стажёра наставнику «что-то не так» и ответ наставника (store.Signal). Именные.
-- text и reply — с замаскированными персональными данными, до 500 символов.
CREATE TABLE mentor_signals (
    id          TEXT PRIMARY KEY,               -- SIG-000001
    team_id     TEXT NOT NULL REFERENCES teams(id),
    user_id     TEXT NOT NULL REFERENCES users(id),
    text        TEXT NOT NULL,
    topic       TEXT NOT NULL CHECK (topic IN ('blocked','tech','unclear','other')),
    at          TIMESTAMPTZ NOT NULL,
    done        BOOLEAN NOT NULL DEFAULT FALSE,
    reply       TEXT CHECK (char_length(reply) <= 500),
    resolved_by TEXT,                           -- имя наставника или руководителя
    resolved_at TIMESTAMPTZ
);

-- «Пульс»: одна отметка самочувствия стажёра в день (store.PulseEntry). Дата — по TZ_NAME (Europe/Moscow).
CREATE TABLE pulse_entries (
    user_id     TEXT NOT NULL REFERENCES users(id),
    date        DATE NOT NULL,
    mood        INT  NOT NULL CHECK (mood BETWEEN 1 AND 5),
    note        TEXT CHECK (char_length(note) <= 300), -- видят наставник и руководитель; PII маскируются
    at          TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, date)
);

-- Честные счётчики «Сэкономлено наставнику» (store.TeamStats):
-- closed = kb_answers + glossary + confirmed_actions; меню, small talk, чек-лист и черновики не считаются,
-- подтверждённая и затем отменённая заявка списывается.
CREATE TABLE team_stats (
    team_id           TEXT PRIMARY KEY REFERENCES teams(id),
    closed            BIGINT NOT NULL DEFAULT 0,
    kb_answers        BIGINT NOT NULL DEFAULT 0,
    glossary          BIGINT NOT NULL DEFAULT 0,
    confirmed_actions BIGINT NOT NULL DEFAULT 0,
    signals           BIGINT NOT NULL DEFAULT 0,
    refusals          BIGINT NOT NULL DEFAULT 0
);

-- Личные счётчики для достижений (store.UserStats). Сами достижения выводятся из состояния, не хранятся.
CREATE TABLE user_stats (
    user_id           TEXT PRIMARY KEY REFERENCES users(id),
    questions         BIGINT NOT NULL DEFAULT 0,
    first_question_at TIMESTAMPTZ,
    first_signal_at   TIMESTAMPTZ,
    first_signal_team TEXT REFERENCES teams(id)
);
-- Демо-сброс (POST /api/v1/demo/reset, только DEMO_MODE=true) = TRUNCATE всех таблиц состояния + сид чек-листов.

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
