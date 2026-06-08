-- 003_devices_channels.sql: Expandir devices + channels

-- ═══════════════════════════════════════════════════
-- EXPANDIR DEVICES
-- ═══════════════════════════════════════════════════

ALTER TABLE devices
    ADD COLUMN IF NOT EXISTS zone_id UUID REFERENCES zones(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS device_type device_type_enum NOT NULL DEFAULT 'actuator_node',
    ADD COLUMN IF NOT EXISTS firmware_version VARCHAR(50),
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_devices_zone ON devices(zone_id);

-- ═══════════════════════════════════════════════════
-- DEVICE CREDENTIALS
-- ═══════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS device_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    credential_type VARCHAR(50) NOT NULL DEFAULT 'mqtt_password',
    credential_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_device_credentials_device ON device_credentials(device_id);

-- ═══════════════════════════════════════════════════
-- SENSOR CHANNELS
-- ═══════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS sensor_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    channel_key VARCHAR(100) NOT NULL,
    label VARCHAR(255) NOT NULL,
    unit VARCHAR(20),
    min_value DOUBLE PRECISION,
    max_value DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(device_id, channel_key)
);

CREATE INDEX IF NOT EXISTS idx_sensor_channels_device ON sensor_channels(device_id);

-- ═══════════════════════════════════════════════════
-- ACTUATOR CHANNELS
-- ═══════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS actuator_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    channel_key VARCHAR(100) NOT NULL,
    label VARCHAR(255) NOT NULL,
    actuator_type actuator_type_enum NOT NULL DEFAULT 'valve',
    max_runtime_sec INTEGER,
    mutex_group VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(device_id, channel_key)
);

CREATE INDEX IF NOT EXISTS idx_actuator_channels_device ON actuator_channels(device_id);
