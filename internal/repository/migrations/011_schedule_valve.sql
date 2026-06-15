-- 011_schedule_valve.sql: Adicionar valve_number aos agendamentos

-- Adicionar coluna valve_number (1-4) para identificar qual válvula este agendamento controla
ALTER TABLE schedules ADD COLUMN IF NOT EXISTS valve_number INTEGER NOT NULL DEFAULT 1;

-- Tornar zone_id nullable (já que as zonas foram removidas na migração 010)
ALTER TABLE schedules ALTER COLUMN zone_id DROP NOT NULL;
