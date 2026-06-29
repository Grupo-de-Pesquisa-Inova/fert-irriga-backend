-- 013_manual_commands_device_id.sql: manual_commands.device_id usa o identificador MQTT
--
-- A API e o MQTT trabalham com devices.device_id (ex.: esp32-001). A migration
-- 006 criou manual_commands.device_id como UUID referenciando devices.id, o que
-- fazia o endpoint /commands inserir "esp32-001" em uma coluna UUID.

ALTER TABLE manual_commands
  DROP CONSTRAINT IF EXISTS manual_commands_device_id_fkey;

ALTER TABLE manual_commands
  DROP CONSTRAINT IF EXISTS manual_commands_device_mqtt_fkey;

ALTER TABLE manual_commands
  ALTER COLUMN device_id TYPE VARCHAR(64) USING device_id::text;

ALTER TABLE manual_commands
  ADD CONSTRAINT manual_commands_device_mqtt_fkey
  FOREIGN KEY (device_id) REFERENCES devices(device_id) ON DELETE CASCADE
  NOT VALID;
