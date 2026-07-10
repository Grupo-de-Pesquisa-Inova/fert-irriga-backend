-- 014_alarm_clock_desync.sql: alarme quando o RTC do ESP32 não sincronizou via NTP
--
-- Enquanto o campo seguranca.relogio_dessincronizado vier true no payload,
-- toda telemetria é classificada como "histórica" no backend (ver
-- ProcessTelemetry/isHistorical) e o estado do device para de atualizar.
-- zone_id NULL replica o padrão do alarme "Falha de Comunicação": aplica
-- globalmente, a todos os devices.

INSERT INTO alarm_rules (zone_id, name, condition_type, channel_key, threshold_value, duration_sec, severity)
VALUES (NULL, 'Relógio do ESP32 Dessincronizado', 'clock_desync', NULL, NULL, 300, 'warning')
ON CONFLICT DO NOTHING;
