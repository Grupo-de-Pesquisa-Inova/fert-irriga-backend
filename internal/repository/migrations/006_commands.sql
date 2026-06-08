-- 006_commands.sql: Comandos manuais com rastreabilidade completa

CREATE TABLE IF NOT EXISTS manual_commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    command_id VARCHAR(64) UNIQUE NOT NULL,
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    origin command_origin_enum NOT NULL DEFAULT 'web_manual',
    actor VARCHAR(255) NOT NULL DEFAULT 'system',
    action VARCHAR(100) NOT NULL,
    target_channel VARCHAR(100),
    parameters JSONB NOT NULL DEFAULT '{}',
    status command_status_enum NOT NULL DEFAULT 'pending',
    safety_context JSONB,
    priority INTEGER NOT NULL DEFAULT 4,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    dispatched_at TIMESTAMPTZ,
    acked_at TIMESTAMPTZ,
    expired_at TIMESTAMPTZ,
    result JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_manual_commands_device ON manual_commands(device_id, requested_at DESC);
CREATE INDEX IF NOT EXISTS idx_manual_commands_status ON manual_commands(status);
CREATE INDEX IF NOT EXISTS idx_manual_commands_cmd_id ON manual_commands(command_id);
