-- e9-1-prod-usage.sql — Story 9-1 tier-3 modality usage audit.
--
-- Run via: psql "$PG_DSN" -f scripts/e9-1-prod-usage.sql
-- (or via wrapper: bash scripts/story-9-1-tier3-audit-drill.sh)
--
-- Schema notes (sourced from internal/domain/entity/log.go and channel.go):
--   * Consume log rows live in TABLE `logs` with `type=2` (LogTypeConsume);
--     the audit doc calls this "consume_logs" but the physical table is `logs`.
--   * Column is `model_name` (not `model`).
--   * `created_at` is bigint epoch seconds (not timestamp). 30d ago = now - 30*86400.
--   * `channels.status = 1` means enabled (constants/channel.go).
--
-- Tier-3 model name patterns sourced from:
--   * internal/pkg/setting/ratio_setting/model_ratio.go:280-307 (mj_*, suno_*, sora-*)
--   * internal/pkg/constant/task.go:5-9 (TaskPlatform: suno/music/mj)
--   * internal/pkg/constant/channel.go:50-55 (Kling/Jimeng/Vidu/DoubaoVideo/Sora)
--
-- Tier-3 channel types (story-9-1 audit doc, channel.go):
--   2 = Midjourney, 5 = MidjourneyPlus, 36 = SunoAPI,
--   50 = Kling, 51 = Jimeng, 52 = Vidu, 54 = DoubaoVideo, 55 = Sora

\timing on
\echo === Query 1: requests + total quota per tier-3 model (last 30d) ===

SELECT
    model_name,
    COUNT(*)            AS requests,
    SUM(quota)::bigint  AS total_quota
FROM logs
WHERE type = 2
  AND created_at >= EXTRACT(EPOCH FROM (NOW() - INTERVAL '30 days'))::bigint
  AND (
        model_name LIKE 'mj_%'
     OR model_name LIKE 'mj-%'
     OR model_name LIKE 'midjourney%'
     OR model_name = 'swap_face'
     OR model_name LIKE 'suno_%'
     OR model_name LIKE 'suno-%'
     OR model_name LIKE 'music%'
     OR model_name LIKE 'sora-%'
     OR model_name LIKE 'kling%'
     OR model_name LIKE 'jimeng%'
     OR model_name LIKE 'vidu%'
     OR model_name LIKE 'hailuo%'
     OR model_name LIKE '%video%'
     OR model_name LIKE '%realtime%'
  )
GROUP BY model_name
ORDER BY total_quota DESC NULLS LAST;

\echo
\echo === Query 2: total quota per tier-3 category (last 30d) ===

SELECT
    'MJ'        AS category,
    SUM(quota)::bigint AS total_quota,
    COUNT(*)           AS requests
FROM logs
WHERE type = 2
  AND created_at >= EXTRACT(EPOCH FROM (NOW() - INTERVAL '30 days'))::bigint
  AND (model_name LIKE 'mj_%' OR model_name LIKE 'mj-%'
       OR model_name LIKE 'midjourney%' OR model_name = 'swap_face')
UNION ALL
SELECT
    'Suno',
    SUM(quota)::bigint,
    COUNT(*)
FROM logs
WHERE type = 2
  AND created_at >= EXTRACT(EPOCH FROM (NOW() - INTERVAL '30 days'))::bigint
  AND (model_name LIKE 'suno_%' OR model_name LIKE 'suno-%')
UNION ALL
SELECT
    'Music',
    SUM(quota)::bigint,
    COUNT(*)
FROM logs
WHERE type = 2
  AND created_at >= EXTRACT(EPOCH FROM (NOW() - INTERVAL '30 days'))::bigint
  AND model_name LIKE 'music%'
UNION ALL
SELECT
    'Video',
    SUM(quota)::bigint,
    COUNT(*)
FROM logs
WHERE type = 2
  AND created_at >= EXTRACT(EPOCH FROM (NOW() - INTERVAL '30 days'))::bigint
  AND (model_name LIKE 'sora-%' OR model_name LIKE 'kling%'
       OR model_name LIKE 'jimeng%' OR model_name LIKE 'vidu%'
       OR model_name LIKE 'hailuo%' OR model_name LIKE '%video%')
UNION ALL
SELECT
    'Realtime',
    SUM(quota)::bigint,
    COUNT(*)
FROM logs
WHERE type = 2
  AND created_at >= EXTRACT(EPOCH FROM (NOW() - INTERVAL '30 days'))::bigint
  AND model_name LIKE '%realtime%'
ORDER BY total_quota DESC NULLS LAST;

\echo
\echo === Query 3: active channels by tier-3 type (status=1=enabled) ===

SELECT
    type,
    CASE type
        WHEN 2  THEN 'Midjourney'
        WHEN 5  THEN 'MidjourneyPlus'
        WHEN 36 THEN 'SunoAPI'
        WHEN 50 THEN 'Kling'
        WHEN 51 THEN 'Jimeng'
        WHEN 52 THEN 'Vidu'
        WHEN 54 THEN 'DoubaoVideo'
        WHEN 55 THEN 'Sora'
        ELSE 'Unknown'
    END                                            AS channel_name,
    COUNT(*)                                       AS total_channels,
    COUNT(*) FILTER (WHERE status = 1)             AS enabled_channels
FROM channels
WHERE type IN (2, 5, 36, 50, 51, 52, 54, 55)
GROUP BY type
ORDER BY type;
