-- 010_cleanup.sql: Remover tabelas enterprise não utilizadas
--
-- Contexto: o backend foi criado com arquitetura multi-tenant (Organizações →
-- Sites → Estufas → Zonas → Devices + Receitas + Auth), mas o escopo real é de
-- 1 ESP32. Esta migração remove as tabelas mortas.
--
-- Observações importantes (não alterar sem entender o impacto):
--   * DROP TABLE ... CASCADE remove apenas as constraints/objetos que DEPENDEM
--     da tabela (ex.: as FKs zone_id em schedules, alarm_rules e devices), e NÃO
--     as tabelas que mantemos. Logo schedules, alarm_rules e devices sobrevivem
--     — apenas perdem a FK para zones/recipes.
--   * device_type_enum NÃO é dropado: a tabela devices (mantida) ainda o utiliza
--     na coluna device_type.

-- Tabelas de auth (dependem de organizations)
DROP TABLE IF EXISTS user_roles CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Tabelas de receitas (schedules.recipe_id perde a FK, mas a tabela permanece)
DROP TABLE IF EXISTS recipe_steps CASCADE;
DROP TABLE IF EXISTS recipes CASCADE;

-- Hierarquia enterprise (schedules.zone_id, alarm_rules.zone_id e devices.zone_id
-- perdem a FK, mas as tabelas permanecem)
DROP TABLE IF EXISTS zones CASCADE;
DROP TABLE IF EXISTS greenhouses CASCADE;
DROP TABLE IF EXISTS sites CASCADE;
DROP TABLE IF EXISTS organizations CASCADE;

-- Tabelas de canais individuais (ESP32 envia tudo como blob JSON)
DROP TABLE IF EXISTS sensor_channels CASCADE;
DROP TABLE IF EXISTS actuator_channels CASCADE;
DROP TABLE IF EXISTS device_credentials CASCADE;

-- Enums órfãos — usados somente por tabelas removidas acima.
-- device_type_enum é preservado (devices ainda o usa).
DROP TYPE IF EXISTS recipe_type_enum CASCADE;
DROP TYPE IF EXISTS actuator_type_enum CASCADE;
