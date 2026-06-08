-- 005_schedules.sql: Agendamentos e histórico de execuções

CREATE TABLE IF NOT EXISTS schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id UUID NOT NULL REFERENCES zones(id) ON DELETE CASCADE,
    recipe_id UUID REFERENCES recipes(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    schedule_type schedule_type_enum NOT NULL DEFAULT 'one_time',
    cron_expression VARCHAR(100),
    start_at TIMESTAMPTZ,
    start_window_min INTEGER NOT NULL DEFAULT 5,
    duration_sec INTEGER NOT NULL DEFAULT 0,
    origin command_origin_enum NOT NULL DEFAULT 'web_manual',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    version INTEGER NOT NULL DEFAULT 1,
    next_execution_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_schedules_zone ON schedules(zone_id);
CREATE INDEX IF NOT EXISTS idx_schedules_next ON schedules(next_execution_at) WHERE is_enabled = true;

CREATE TABLE IF NOT EXISTS schedule_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    schedule_id UUID NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    status run_status_enum NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_schedule_runs_schedule ON schedule_runs(schedule_id);
