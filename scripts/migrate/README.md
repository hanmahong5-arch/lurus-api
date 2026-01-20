# MySQL to PostgreSQL Migration Guide / MySQL 迁移到 PostgreSQL 指南

## Overview / 概述

This guide helps you migrate your lurus-api database from MySQL to PostgreSQL.

本指南帮助你将 lurus-api 数据库从 MySQL 迁移到 PostgreSQL。

## Prerequisites / 前提条件

1. **PostgreSQL Server** - Install and configure PostgreSQL (recommended version: 14+)
2. **Go 1.18+** - For running the migration tool
3. **Backup** - Always backup your MySQL database before migration!

1. **PostgreSQL 服务器** - 安装并配置 PostgreSQL（推荐版本：14+）
2. **Go 1.18+** - 用于运行迁移工具
3. **备份** - 迁移前务必备份 MySQL 数据库！

## Step 1: Create PostgreSQL Database / 创建 PostgreSQL 数据库

```sql
-- Connect to PostgreSQL as superuser
-- 以超级用户身份连接 PostgreSQL
CREATE DATABASE new_api;
CREATE USER new_api_user WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE new_api TO new_api_user;

-- Connect to the new database and grant schema permissions
-- 连接到新数据库并授予 schema 权限
\c new_api
GRANT ALL ON SCHEMA public TO new_api_user;
```

## Step 2: Initialize PostgreSQL Schema / 初始化 PostgreSQL Schema

The easiest way is to let GORM auto-migrate the schema. Start lurus-api with PostgreSQL DSN once:

最简单的方式是让 GORM 自动迁移 schema。使用 PostgreSQL DSN 启动 lurus-api 一次：

```bash
# Set PostgreSQL DSN temporarily
# 临时设置 PostgreSQL DSN
export SQL_DSN="postgres://new_api_user:your_password@localhost:5432/new_api"

# Start lurus-api to auto-create tables
# 启动 lurus-api 自动创建表
./lurus-api

# Stop after tables are created (Ctrl+C)
# 表创建后停止（Ctrl+C）
```

## Step 3: Run Migration Tool / 运行迁移工具

```bash
cd scripts/migrate

# Build the migration tool
# 编译迁移工具
go build -o migrate.exe .

# Run migration
# 运行迁移
./migrate.exe \
  -mysql "user:password@tcp(localhost:3306)/one_api?parseTime=true" \
  -pg "postgres://new_api_user:password@localhost:5432/new_api" \
  -batch 1000 \
  -truncate
```

### Migration Options / 迁移选项

| Option | Description (EN) | 描述 (ZH) |
|--------|-----------------|-----------|
| `-mysql` | MySQL DSN | MySQL 连接字符串 |
| `-pg` | PostgreSQL DSN | PostgreSQL 连接字符串 |
| `-batch` | Batch size for inserts (default: 1000) | 批量插入大小（默认：1000） |
| `-tables` | Comma-separated list of tables to migrate | 要迁移的表（逗号分隔） |
| `-truncate` | Truncate target tables before insert | 插入前清空目标表 |
| `-dry-run` | Only show what would be done | 仅显示将要执行的操作 |

### Example: Migrate Specific Tables / 示例：迁移特定表

```bash
# Migrate only users and tokens tables
# 仅迁移 users 和 tokens 表
./migrate.exe \
  -mysql "root:123456@tcp(localhost:3306)/one_api?parseTime=true" \
  -pg "postgres://postgres:123456@localhost:5432/new_api" \
  -tables "users,tokens"
```

## Step 4: Update Configuration / 更新配置

Update your `.env` file or environment variables:

更新你的 `.env` 文件或环境变量：

```bash
# Before (MySQL) / 之前（MySQL）
# SQL_DSN=root:123456@tcp(localhost:3306)/one_api?parseTime=true

# After (PostgreSQL) / 之后（PostgreSQL）
SQL_DSN=postgres://new_api_user:password@localhost:5432/new_api

# Optional: Separate log database
# 可选：独立日志数据库
# LOG_SQL_DSN=postgres://new_api_user:password@localhost:5432/new_api_logs
```

## Step 5: Verify Migration / 验证迁移

```bash
# Start lurus-api with new PostgreSQL configuration
# 使用新的 PostgreSQL 配置启动 lurus-api
./lurus-api

# Check logs for any errors
# 检查日志是否有错误
```

## Key Differences Between MySQL and PostgreSQL / MySQL 和 PostgreSQL 的主要差异

| Feature | MySQL | PostgreSQL |
|---------|-------|------------|
| Boolean values | 1/0 | true/false |
| Identifier quoting | \`backticks\` | "double quotes" |
| Case sensitivity | Case-insensitive by default | Case-sensitive |
| JSON type | JSON | JSONB (more features) |
| AUTO_INCREMENT | AUTO_INCREMENT | SERIAL or IDENTITY |

The lurus-api codebase already handles these differences automatically.

lurus-api 代码库已经自动处理了这些差异。

## Troubleshooting / 故障排除

### Connection Issues / 连接问题

```bash
# Test PostgreSQL connection
# 测试 PostgreSQL 连接
psql "postgres://user:password@localhost:5432/new_api" -c "SELECT 1"
```

### Permission Issues / 权限问题

```sql
-- Grant all permissions
-- 授予所有权限
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO new_api_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO new_api_user;
```

### Sequence Reset (Auto-increment) / 序列重置（自增）

After migration, reset sequences to avoid ID conflicts:

迁移后重置序列以避免 ID 冲突：

```sql
-- Example for users table
-- users 表示例
SELECT setval('users_id_seq', (SELECT MAX(id) FROM users));

-- Or reset all sequences
-- 或重置所有序列
SELECT setval(pg_get_serial_sequence(t.table_name, c.column_name),
              (SELECT MAX(c.column_name::text::bigint) FROM t.table_name))
FROM information_schema.tables t
JOIN information_schema.columns c ON c.table_name = t.table_name
WHERE c.column_default LIKE 'nextval%'
  AND t.table_schema = 'public';
```

## Rollback / 回滚

If you need to rollback to MySQL:

如果需要回滚到 MySQL：

1. Stop lurus-api / 停止 lurus-api
2. Change `SQL_DSN` back to MySQL DSN / 将 `SQL_DSN` 改回 MySQL DSN
3. Start lurus-api / 启动 lurus-api

Your MySQL database should still have all the original data (if you didn't modify it).

你的 MySQL 数据库应该仍然保留所有原始数据（如果没有修改）。

## Notes / 注意事项

- **Large databases**: For databases with millions of rows, consider increasing batch size and running during off-peak hours.
- **大型数据库**：对于百万级行数的数据库，考虑增加批量大小并在非高峰期运行。

- **JSON fields**: The migration tool preserves JSON data as-is. PostgreSQL's JSONB type provides additional query capabilities.
- **JSON 字段**：迁移工具原样保留 JSON 数据。PostgreSQL 的 JSONB 类型提供额外的查询功能。

- **Soft deletes**: Records with `deleted_at` values are also migrated.
- **软删除**：带有 `deleted_at` 值的记录也会被迁移。
