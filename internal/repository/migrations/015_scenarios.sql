CREATE TABLE scenario_plans (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 device_id TEXT NOT NULL,
 scenario_id TEXT NOT NULL,
 name TEXT NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
 duration_sec INTEGER NOT NULL CHECK (duration_sec BETWEEN 1 AND 7200),
 start_at TIMESTAMPTZ NOT NULL,
 repeat TEXT NOT NULL CHECK (repeat IN ('once', 'daily')),
 enabled BOOLEAN NOT NULL DEFAULT true,
 next_at TIMESTAMPTZ,
 deleted_at TIMESTAMPTZ
);
CREATE INDEX scenario_plans_due ON scenario_plans(next_at) WHERE enabled AND deleted_at IS NULL;
CREATE TABLE scenario_runs (
 id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
 plan_id UUID REFERENCES scenario_plans(id),
 device_id TEXT NOT NULL,
 scenario_id TEXT NOT NULL,
 duration_sec INTEGER NOT NULL CHECK (duration_sec BETWEEN 1 AND 7200),
 due_at TIMESTAMPTZ NOT NULL,
 status TEXT NOT NULL CHECK (status IN ('pending','running','stopping','uncertain','completed','cancelled','failed')),
 error_code TEXT NOT NULL DEFAULT '',
 message TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 started_at TIMESTAMPTZ,
 ends_at TIMESTAMPTZ,
 finished_at TIMESTAMPTZ,
 UNIQUE(plan_id, due_at)
);
CREATE UNIQUE INDEX scenario_runs_device_active ON scenario_runs(device_id)
 WHERE status IN ('pending','running','stopping','uncertain');
CREATE INDEX scenario_runs_history ON scenario_runs(device_id, created_at DESC);
