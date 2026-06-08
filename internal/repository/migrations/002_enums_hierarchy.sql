-- 002_enums_hierarchy.sql: Enums + Hierarquia organizacional

-- ═══════════════════════════════════════════════════
-- ENUMS
-- ═══════════════════════════════════════════════════

CREATE TYPE device_type_enum AS ENUM ('sensor_node', 'actuator_node', 'gateway');
CREATE TYPE actuator_type_enum AS ENUM ('valve', 'pump', 'relay', 'contactor');
CREATE TYPE recipe_type_enum AS ENUM ('irrigation', 'fertigation', 'flush', 'mixing');
CREATE TYPE schedule_type_enum AS ENUM ('one_time', 'recurring');
CREATE TYPE command_origin_enum AS ENUM ('local_manual', 'web_manual', 'local_schedule', 'cloud_schedule', 'system_automation', 'maintenance');
CREATE TYPE command_status_enum AS ENUM ('pending', 'dispatched', 'received', 'executing', 'executed', 'rejected', 'expired', 'conflicted', 'failed');
CREATE TYPE run_status_enum AS ENUM ('pending', 'running', 'completed', 'failed', 'cancelled');
CREATE TYPE alarm_severity_enum AS ENUM ('info', 'warning', 'critical', 'emergency');
CREATE TYPE alarm_event_status_enum AS ENUM ('active', 'acknowledged', 'resolved');

-- ═══════════════════════════════════════════════════
-- ORGANIZATIONS
-- ═══════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- ═══════════════════════════════════════════════════
-- SITES
-- ═══════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS sites (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    location VARCHAR(500),
    timezone VARCHAR(50) NOT NULL DEFAULT 'America/Sao_Paulo',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_sites_org ON sites(organization_id);

-- ═══════════════════════════════════════════════════
-- GREENHOUSES
-- ═══════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS greenhouses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_greenhouses_site ON greenhouses(site_id);

-- ═══════════════════════════════════════════════════
-- ZONES
-- ═══════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS zones (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    greenhouse_id UUID NOT NULL REFERENCES greenhouses(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_zones_greenhouse ON zones(greenhouse_id);
