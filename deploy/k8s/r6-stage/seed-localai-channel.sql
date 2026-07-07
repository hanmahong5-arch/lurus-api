-- Track 1.2 — newhub channel seed: route an OpenAI-compatible model name to the
-- in-cluster LocalAI private-inference pod (ns lurus-inference). Idempotent.
-- ============================================================================
-- After this seed + a channel-cache sync, callers hitting the gateway with
--   model = "qwen2.5-7b-instruct"
-- are relayed to LocalAI over the cluster network — "私有推理" with zero data
-- egress. This is a DATA seed (no schema change), mirroring seed-default-tenant.sql.
--
-- 用法:
--   ssh root@100.122.83.20 "kubectl exec -i -n database lurus-pg-0 -- \
--     psql -U lurus -d newhub" < seed-localai-channel.sql
--   然后触发渠道缓存同步(渠道列表页「刷新」或重启 pod)。
--
-- 幂等:WHERE NOT EXISTS(channels.name) — channels 无自然唯一键,故按 name 去重;
--       重跑无副作用。
--
-- ⚠️ base_url 不带 /v1:new-api/newhub 对 OpenAI/Custom 渠道自动追加 /v1/...。
--    若部署版本行为不同(直拼 base_url),改成带 /v1 再跑。
-- ⚠️ Type 8 = Custom(OpenAI 兼容自定义端点,见 internal/pkg/constant/channel.go:12)。
-- ⚠️ key 占位:LocalAI 忽略鉴权;留非空占位满足 NOT NULL + SDK 习惯。

INSERT INTO channels (type, key, name, base_url, models, "group", status, weight, priority, created_time, test_time, channel_info)
SELECT
    8,                                                  -- ChannelTypeCustom
    'sk-localai-internal',                              -- placeholder (LocalAI ignores)
    'LocalAI (private inference)',
    'http://localai.lurus-inference.svc:8080',          -- NO trailing /v1 (gateway appends)
    'qwen2.5-7b-instruct',                              -- must match LocalAI served model id
    'default',
    1,                                                  -- status: 1 = enabled
    0,
    0,
    EXTRACT(EPOCH FROM NOW())::bigint,
    0,
    '{}'
WHERE NOT EXISTS (
    SELECT 1 FROM channels WHERE name = 'LocalAI (private inference)'
);

-- Verify
SELECT id, type, name, base_url, models, status
FROM channels
WHERE name = 'LocalAI (private inference)';
