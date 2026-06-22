-- 012: ingestão idempotente de telemetria (store-and-forward do ESP32)
--
-- Ao reenviar a fila offline, o ESP32 pode repetir registros. Uma chave única
-- (device_id, recorded_at) + ON CONFLICT DO NOTHING evita duplicatas.

-- Remove duplicatas pré-existentes (mantém o registro de maior id por grupo).
DELETE FROM telemetry a
USING telemetry b
WHERE a.id < b.id
  AND a.device_id = b.device_id
  AND a.recorded_at = b.recorded_at;

CREATE UNIQUE INDEX IF NOT EXISTS telemetry_device_recorded_uidx
  ON telemetry (device_id, recorded_at);
