-- PostgreSQL Database Initialization Script for new-api
-- PostgreSQL 数据库初始化脚本

-- 1. Create database (run as superuser / 以超级用户身份运行)
-- CREATE DATABASE new_api;

-- 2. Create user and grant permissions (optional)
-- CREATE USER new_api_user WITH PASSWORD 'your_secure_password';
-- GRANT ALL PRIVILEGES ON DATABASE new_api TO new_api_user;

-- 3. Connect to the new_api database and run the following:
-- \c new_api

-- Grant schema permissions
GRANT ALL ON SCHEMA public TO new_api_user;

-- After migration, reset all sequences to avoid ID conflicts
-- 迁移后重置所有序列以避免 ID 冲突

-- Reset users sequence
SELECT setval(pg_get_serial_sequence('users', 'id'), COALESCE((SELECT MAX(id) FROM users), 0) + 1, false);

-- Reset channels sequence
SELECT setval(pg_get_serial_sequence('channels', 'id'), COALESCE((SELECT MAX(id) FROM channels), 0) + 1, false);

-- Reset tokens sequence
SELECT setval(pg_get_serial_sequence('tokens', 'id'), COALESCE((SELECT MAX(id) FROM tokens), 0) + 1, false);

-- Reset redemptions sequence
SELECT setval(pg_get_serial_sequence('redemptions', 'id'), COALESCE((SELECT MAX(id) FROM redemptions), 0) + 1, false);

-- Reset logs sequence
SELECT setval(pg_get_serial_sequence('logs', 'id'), COALESCE((SELECT MAX(id) FROM logs), 0) + 1, false);

-- Reset midjourneys sequence
SELECT setval(pg_get_serial_sequence('midjourneys', 'id'), COALESCE((SELECT MAX(id) FROM midjourneys), 0) + 1, false);

-- Reset top_ups sequence
SELECT setval(pg_get_serial_sequence('top_ups', 'id'), COALESCE((SELECT MAX(id) FROM top_ups), 0) + 1, false);

-- Reset quota_data sequence
SELECT setval(pg_get_serial_sequence('quota_data', 'id'), COALESCE((SELECT MAX(id) FROM quota_data), 0) + 1, false);

-- Reset tasks sequence
SELECT setval(pg_get_serial_sequence('tasks', 'id'), COALESCE((SELECT MAX(id) FROM tasks), 0) + 1, false);

-- Reset models sequence
SELECT setval(pg_get_serial_sequence('models', 'id'), COALESCE((SELECT MAX(id) FROM models), 0) + 1, false);

-- Reset vendors sequence
SELECT setval(pg_get_serial_sequence('vendors', 'id'), COALESCE((SELECT MAX(id) FROM vendors), 0) + 1, false);

-- Reset prefill_groups sequence
SELECT setval(pg_get_serial_sequence('prefill_groups', 'id'), COALESCE((SELECT MAX(id) FROM prefill_groups), 0) + 1, false);

-- Reset setups sequence
SELECT setval(pg_get_serial_sequence('setups', 'id'), COALESCE((SELECT MAX(id) FROM setups), 0) + 1, false);

-- Reset two_fas sequence
SELECT setval(pg_get_serial_sequence('two_fas', 'id'), COALESCE((SELECT MAX(id) FROM two_fas), 0) + 1, false);

-- Reset two_fa_backup_codes sequence
SELECT setval(pg_get_serial_sequence('two_fa_backup_codes', 'id'), COALESCE((SELECT MAX(id) FROM two_fa_backup_codes), 0) + 1, false);

-- Reset checkins sequence
SELECT setval(pg_get_serial_sequence('checkins', 'id'), COALESCE((SELECT MAX(id) FROM checkins), 0) + 1, false);

-- Reset passkey_credentials sequence
SELECT setval(pg_get_serial_sequence('passkey_credentials', 'id'), COALESCE((SELECT MAX(id) FROM passkey_credentials), 0) + 1, false);

-- Verify sequences (optional verification query)
-- 验证序列（可选验证查询）
-- SELECT c.relname AS sequence_name,
--        pg_get_serial_sequence(t.relname::text, a.attname::text) AS full_name,
--        (SELECT last_value FROM pg_sequences WHERE schemaname = 'public' AND sequencename = c.relname) AS current_value
-- FROM pg_class c
-- JOIN pg_namespace n ON n.oid = c.relnamespace
-- WHERE c.relkind = 'S' AND n.nspname = 'public';

SELECT 'All sequences reset successfully!' AS status;
