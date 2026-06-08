-- 007_alarms.sql: Regras de alarme e eventos

CREATE TABLE IF NOT EXISTS alarm_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id UUID REFERENCES zones(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    condition_type VARCHAR(100) NOT NULL,
    channel_key VARCHAR(100),
    threshold_value DOUBLE PRECISION,
    duration_sec INTEGER,
    severity alarm_severity_enum NOT NULL DEFAULT 'warning',
    is_enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alarm_rules_zone ON alarm_rules(zone_id);

CREATE TABLE IF NOT EXISTS alarm_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alarm_rule_id UUID NOT NULL REFERENCES alarm_rules(id) ON DELETE CASCADE,
    device_id UUID REFERENCES devices(id) ON DELETE SET NULL,
    status alarm_event_status_enum NOT NULL DEFAULT 'active',
    triggered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMPTZ,
    acknowledged_by UUID,
    resolved_at TIMESTAMPTZ,
    context JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_alarm_events_status ON alarm_events(status);
CREATE INDEX IF NOT EXISTS idx_alarm_events_rule ON alarm_events(alarm_rule_id);
