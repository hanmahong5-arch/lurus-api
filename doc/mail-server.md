# Stalwart Mail Server Integration

Lurus Switch integrates Stalwart Mail Server (enterprise mail + collaboration): Email (SMTP/IMAP/POP3/JMAP), Collaboration (CalDAV/CardDAV/WebDAV), Security (SPF/DKIM/DMARC/ARC/DANE/MTA-STS), anti-spam (Bayesian + DNSBL + greylisting). Clients (Outlook/Thunderbird/Apple Mail/mobile/JMAP) → Stalwart → RocksDB or PostgreSQL unified storage.

## Quick Start

```bash
cd deploy/stalwart
.\setup-mail.ps1                                          # Windows dev (admin/changeme @ http://localhost:8080)
./setup-mail.sh                                           # Linux/macOS dev
.\setup-mail.ps1 -Domain "yourdomain.com" -Production    # production
./setup-mail.sh --domain yourdomain.com --production
```

## DNS (for yourdomain.com)

| Type | Name | Value | Priority |
|------|------|-------|----------|
| A | mail | `<SERVER_IP>` | - |
| MX | @ | mail.yourdomain.com | 10 |
| TXT | @ | `v=spf1 mx ~all` | - |
| TXT | _dmarc | `v=DMARC1; p=quarantine; rua=mailto:admin@yourdomain.com` | - |
| TXT | default._domainkey | `<DKIM_PUBLIC_KEY>` | - |

Generate DKIM after Stalwart runs: Web Admin → Settings → Domain → DKIM → copy TXT value.

## Ports

| Port | Protocol | |
|------|----------|---|
| 25 | SMTP | MTA |
| 465 | SMTPS | implicit TLS |
| 587 | Submission | STARTTLS |
| 143 | IMAP | STARTTLS |
| 993 | IMAPS | implicit TLS |
| 8080 | HTTP | Web Admin + JMAP |
| 443 | HTTPS | Web Admin + JMAP (via Caddy) |

## Config files

`deploy/docker-compose.mail.yml` (dev) · `deploy/docker-compose.mail.ssl.yml` (prod HTTPS) · `deploy/stalwart/{config.toml, Caddyfile.mail, setup-mail.ps1, setup-mail.sh, .env.example}`.

## Storage

Default RocksDB (embedded, single-server, no deps). For multi-server clustering, uncomment PostgreSQL in `config.toml`:

```toml
[store."postgresql"]
type = "postgresql"
host = "postgres"; port = 5432; database = "stalwart"; user = "stalwart"; password = "stalwart123"
```

## Anti-spam (config.toml)

Spam threshold 5.0 (mark) · discard threshold 10.0 (silently drop) · Bayesian auto-learn (Junk folder moves train it) · DNSBLs Spamhaus ZEN + SpamCop.

## Monitoring

Prometheus enabled by default: scrape `http://stalwart:8080/metrics` (`job_name: 'stalwart'`, targets `['stalwart:8080']`). Health: `curl http://localhost:8080/healthz`.

## Common ops

```bash
docker exec stalwart-mail stalwart-cli user create user@domain.com           # add user (or Web Admin → Directory → Users)
docker run --rm -v stalwart_data:/data -v $(pwd):/backup alpine \
  tar czf /backup/stalwart-backup-$(date +%Y%m%d).tar.gz /data               # backup
docker logs -f stalwart-mail                                                  # logs (or setup-mail.{ps1 -Logs, sh --logs})
```

## Troubleshooting

- **Port 25 blocked** (common on cloud): request unblock from provider, or use a relay (AWS SES / SendGrid).
- **Certificate warnings** (self-signed): use production mode with ACME (`--domain yourdomain.com --production`).
- **Connection refused**: `docker ps --filter "name=stalwart"`, `docker logs stalwart-mail`.

## Integration with lurus-api

Stalwart runs as a separate stack. Share the network: start lurus-api (`docker-compose up -d`), then `cd deploy && docker-compose -f docker-compose.mail.yml up -d` (auto-joins `lurus-api_default` network). Both then communicate via Docker network.

> Production DKIM/SMTP config + relay debugging history (DKIM PKCS#8→PKCS#1, Stalwart short-account names, Pod CIDR allowlist) — see `doc/process.md` 2026-02-25 entries.
