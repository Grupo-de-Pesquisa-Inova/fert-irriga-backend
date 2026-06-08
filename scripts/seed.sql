-- seed.sql: Dados demo para desenvolvimento
-- Executar após todas as migrations

-- ═══════════════════════════════════════════════════
-- ORGANIZATION
-- ═══════════════════════════════════════════════════
INSERT INTO organizations (id, name, slug) VALUES
    ('a0000000-0000-0000-0000-000000000001', 'FertIrriga Demo', 'fertirriga-demo')
ON CONFLICT (slug) DO NOTHING;

-- ═══════════════════════════════════════════════════
-- SITES
-- ═══════════════════════════════════════════════════
INSERT INTO sites (id, organization_id, name, location, timezone) VALUES
    ('b0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'Fazenda São João', 'Araquari, SC', 'America/Sao_Paulo')
ON CONFLICT DO NOTHING;

-- ═══════════════════════════════════════════════════
-- GREENHOUSES
-- ═══════════════════════════════════════════════════
INSERT INTO greenhouses (id, site_id, name, description) VALUES
    ('c0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000001', 'Estufa Principal', 'Estufa de 500m² para hortaliças'),
    ('c0000000-0000-0000-0000-000000000002', 'b0000000-0000-0000-0000-000000000001', 'Estufa Experimental', 'Estufa de 200m² para testes')
ON CONFLICT DO NOTHING;

-- ═══════════════════════════════════════════════════
-- ZONES
-- ═══════════════════════════════════════════════════
INSERT INTO zones (id, greenhouse_id, name, description) VALUES
    ('d0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001', 'Zona A - Tomates', 'Irrigação por gotejamento'),
    ('d0000000-0000-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000001', 'Zona B - Alfaces', 'Irrigação por aspersão'),
    ('d0000000-0000-0000-0000-000000000003', 'c0000000-0000-0000-0000-000000000002', 'Zona C - Morangos', 'Hidroponia NFT'),
    ('d0000000-0000-0000-0000-000000000004', 'c0000000-0000-0000-0000-000000000002', 'Zona D - Pimentas', 'Irrigação manual')
ON CONFLICT DO NOTHING;

-- ═══════════════════════════════════════════════════
-- RECIPES
-- ═══════════════════════════════════════════════════
INSERT INTO recipes (id, organization_id, name, description, recipe_type, is_active) VALUES
    ('e0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'Irrigação Padrão 15min', 'Irrigação simples de 15 minutos', 'irrigation', true),
    ('e0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', 'Fertirrigação NPK', 'Fertirrigação com solução NPK em 3 passos', 'fertigation', true)
ON CONFLICT DO NOTHING;

INSERT INTO recipe_steps (recipe_id, step_order, action, target_channel, duration_sec, parameters) VALUES
    ('e0000000-0000-0000-0000-000000000001', 1, 'open_valve', 'irrigacao_conj1', 900, '{}'),
    ('e0000000-0000-0000-0000-000000000001', 2, 'close_valve', 'irrigacao_conj1', 0, '{}'),
    ('e0000000-0000-0000-0000-000000000002', 1, 'open_valve', 'irrigacao_conj1', 120, '{"note":"pré-irrigação"}'),
    ('e0000000-0000-0000-0000-000000000002', 2, 'open_valve', 'adubacao_sol1_bag1', 600, '{"note":"solução NPK"}'),
    ('e0000000-0000-0000-0000-000000000002', 3, 'close_valve', 'adubacao_sol1_bag1', 0, '{}'),
    ('e0000000-0000-0000-0000-000000000002', 4, 'open_valve', 'irrigacao_conj1', 180, '{"note":"lavagem de linha"}'),
    ('e0000000-0000-0000-0000-000000000002', 5, 'close_valve', 'irrigacao_conj1', 0, '{}')
ON CONFLICT DO NOTHING;

-- ═══════════════════════════════════════════════════
-- ALARM RULES
-- ═══════════════════════════════════════════════════
INSERT INTO alarm_rules (zone_id, name, condition_type, channel_key, threshold_value, duration_sec, severity) VALUES
    ('d0000000-0000-0000-0000-000000000001', 'Temperatura Alta', 'threshold_high', 'temperatura_c', 45.0, 60, 'warning'),
    ('d0000000-0000-0000-0000-000000000001', 'Umidade Baixa', 'threshold_low', 'umidade_pct', 20.0, 120, 'warning'),
    (NULL, 'Falha de Comunicação', 'comm_timeout', NULL, NULL, 120, 'critical'),
    ('d0000000-0000-0000-0000-000000000002', 'Pressão Alta', 'threshold_high', 'pressao_hpa', 1050.0, 30, 'critical'),
    ('d0000000-0000-0000-0000-000000000001', 'Falha de Fluxo', 'flow_fail', 'vazao_lpm', 0.0, 60, 'emergency')
ON CONFLICT DO NOTHING;

-- ═══════════════════════════════════════════════════
-- USER ADMIN
-- ═══════════════════════════════════════════════════
-- Por segurança, este seed NÃO cria um usuário administrador com senha
-- padrão. Crie o admin de forma controlada, passando um hash bcrypt gerado
-- a partir de uma senha forte via variável do psql. Exemplo:
--
--   1. Gere o hash (ex.: com o utilitário htpasswd ou um script Go/Node):
--        htpasswd -bnBC 10 "" 'SUA_SENHA_FORTE' | tr -d ':\n'
--
--   2. Rode o seed passando o hash:
--        psql ... -v admin_password_hash="'<HASH_BCRYPT>'" -f seed.sql
--
-- O bloco abaixo só executa quando a variável :admin_password_hash é fornecida.
\if :{?admin_password_hash}
INSERT INTO users (id, organization_id, email, name, password_hash) VALUES
    ('f0000000-0000-0000-0000-000000000001', 'a0000000-0000-0000-0000-000000000001', 'admin@fertirriga.local', 'Administrador', :admin_password_hash)
ON CONFLICT (email) DO NOTHING;

INSERT INTO user_roles (user_id, role_id)
SELECT 'f0000000-0000-0000-0000-000000000001', id FROM roles WHERE name = 'admin'
ON CONFLICT DO NOTHING;
\endif
