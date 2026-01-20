# Stalwart Mail Server Integration

## Overview / 概述

Lurus Switch integrates Stalwart Mail Server as the enterprise mail & collaboration solution, providing:

- **Email**: SMTP, IMAP, POP3, JMAP
- **Collaboration**: CalDAV (Calendar), CardDAV (Contacts), WebDAV (Files)
- **Security**: SPF, DKIM, DMARC, ARC, DANE, MTA-STS
- **Anti-Spam**: Built-in Bayesian filter, DNSBL, greylisting

## Architecture / 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Client Layer                              │
├─────────────┬─────────────┬─────────────┬─────────────┬─────────┤
│  Outlook    │ Thunderbird │   Apple     │   Mobile    │  JMAP   │
│ (Exchange)  │  (IMAP)     │   Mail      │   Clients   │ Clients │
└──────┬──────┴──────┬──────┴──────┬──────┴──────┬──────┴────┬────┘
       │             │             │             │           │
       ▼             ▼             ▼             ▼           ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Stalwart Mail Server                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────────┐│
│  │  SMTP    │ │  IMAP    │ │  JMAP    │ │  CalDAV/CardDAV/     ││
│  │ :25,:587│ │ :143,:993│ │  :8080   │ │  WebDAV              ││
│  │  :465   │ │          │ │          │ │                      ││
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └──────────┬───────────┘│
│       │            │            │                  │            │
│  ┌────▼────────────▼────────────▼──────────────────▼───────────┐│
│  │                    RocksDB / PostgreSQL                      ││
│  │                    (Unified Storage)                         ││
│  └──────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start / 快速开始

### Development Mode / 开发模式

```bash
cd deploy/stalwart

# Windows
.\setup-mail.ps1

# Linux/macOS
./setup-mail.sh
```

Access: http://localhost:8080 (admin / changeme)

### Production Mode / 生产模式

1. Configure DNS records (see below)
2. Run with production flag:

```bash
# Windows
.\setup-mail.ps1 -Domain "yourdomain.com" -Production

# Linux/macOS
./setup-mail.sh --domain yourdomain.com --production
```

## DNS Configuration / DNS 配置

Required DNS records for `yourdomain.com`:

| Type | Name | Value | Priority |
|------|------|-------|----------|
| A | mail | `<SERVER_IP>` | - |
| MX | @ | mail.yourdomain.com | 10 |
| TXT | @ | `v=spf1 mx ~all` | - |
| TXT | _dmarc | `v=DMARC1; p=quarantine; rua=mailto:admin@yourdomain.com` | - |
| TXT | default._domainkey | `<DKIM_PUBLIC_KEY>` | - |

### Generate DKIM Key / 生成 DKIM 密钥

After Stalwart is running:
1. Login to Web Admin
2. Go to Settings > Domain > DKIM
3. Generate key and copy TXT record value

## Ports / 端口

| Port | Protocol | Description |
|------|----------|-------------|
| 25 | SMTP | Mail Transfer Agent (MTA) |
| 465 | SMTPS | SMTP with implicit TLS |
| 587 | Submission | SMTP submission with STARTTLS |
| 143 | IMAP | IMAP with STARTTLS |
| 993 | IMAPS | IMAP with implicit TLS |
| 8080 | HTTP | Web Admin + JMAP |
| 443 | HTTPS | Web Admin + JMAP (with Caddy) |

## Configuration Files / 配置文件

```
deploy/
├── docker-compose.mail.yml       # Development deployment
├── docker-compose.mail.ssl.yml   # Production with HTTPS
└── stalwart/
    ├── config.toml               # Main configuration
    ├── Caddyfile.mail            # Caddy HTTPS proxy
    ├── setup-mail.ps1            # Windows setup script
    ├── setup-mail.sh             # Linux/macOS setup script
    └── .env.example              # Environment template
```

## Storage Options / 存储选项

### Default: RocksDB (Embedded)

Best for single-server deployments. No external dependencies.

### PostgreSQL (Shared)

For multi-server clustering. Uncomment in `config.toml`:

```toml
[store."postgresql"]
type = "postgresql"
host = "postgres"
port = 5432
database = "stalwart"
user = "stalwart"
password = "stalwart123"
```

## Anti-Spam Configuration / 反垃圾邮件配置

Default settings in `config.toml`:

- **Spam threshold**: 5.0 (mark as spam)
- **Discard threshold**: 10.0 (silently discard)
- **Bayesian learning**: Enabled (auto-learn from user actions)
- **DNSBLs**: Spamhaus ZEN, SpamCop

Users can train the filter by moving messages to/from Junk folder.

## Monitoring / 监控

### Prometheus Metrics

Enabled by default. Scrape endpoint: `http://stalwart:8080/metrics`

Add to Prometheus config:

```yaml
scrape_configs:
  - job_name: 'stalwart'
    static_configs:
      - targets: ['stalwart:8080']
```

### Health Check

```bash
curl http://localhost:8080/healthz
```

## Common Operations / 常用操作

### Add User / 添加用户

1. Web Admin > Directory > Users > Add
2. Or via CLI:

```bash
docker exec stalwart-mail stalwart-cli user create user@domain.com
```

### Backup / 备份

```bash
# Backup data volume
docker run --rm -v stalwart_data:/data -v $(pwd):/backup \
  alpine tar czf /backup/stalwart-backup-$(date +%Y%m%d).tar.gz /data
```

### View Logs / 查看日志

```bash
docker logs -f stalwart-mail

# Windows PowerShell
.\setup-mail.ps1 -Logs

# Linux/macOS
./setup-mail.sh --logs
```

## Troubleshooting / 故障排除

### Port 25 Blocked

Many cloud providers block port 25. Solutions:
1. Request port unblock from provider
2. Use a relay service (AWS SES, SendGrid)

### Certificate Issues

For self-signed cert warnings, use production mode with ACME:

```bash
./setup-mail.sh --domain yourdomain.com --production
```

### Connection Refused

Check if Stalwart is running:

```bash
docker ps --filter "name=stalwart"
docker logs stalwart-mail
```

## Integration with lurus-api / 与 lurus-api 集成

Stalwart runs as a separate service stack. To share the network with lurus-api:

```bash
# Start lurus-api first
docker-compose up -d

# Then start Stalwart (auto-joins lurus-api_default network)
cd deploy && docker-compose -f docker-compose.mail.yml up -d
```

Both services can now communicate via Docker network.
