#!/bin/bash
# Stalwart Mail Server Setup Script for Linux/macOS
# Lurus Switch Mail Infrastructure
#
# Usage:
#   ./setup-mail.sh                           # Development mode
#   ./setup-mail.sh --production              # Production with HTTPS
#   ./setup-mail.sh --domain yourdomain.com --production

set -e

# Default values
DOMAIN="lurus.local"
PRODUCTION=false
STOP=false
STATUS=false
LOGS=false

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info() { echo -e "${CYAN}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -d|--domain)
            DOMAIN="$2"
            shift 2
            ;;
        -p|--production)
            PRODUCTION=true
            shift
            ;;
        --stop)
            STOP=true
            shift
            ;;
        --status)
            STATUS=true
            shift
            ;;
        --logs)
            LOGS=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  -d, --domain DOMAIN    Set mail domain (default: lurus.local)"
            echo "  -p, --production       Enable production mode with HTTPS"
            echo "  --stop                 Stop the mail server"
            echo "  --status               Show server status"
            echo "  --logs                 Show server logs"
            echo "  -h, --help             Show this help"
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Navigate to script directory
cd "$(dirname "$0")"

if [ "$STATUS" = true ]; then
    info "Checking Stalwart Mail Server status..."
    docker ps --filter "name=stalwart" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    exit 0
fi

if [ "$LOGS" = true ]; then
    info "Showing Stalwart Mail Server logs..."
    docker logs -f stalwart-mail
    exit 0
fi

if [ "$STOP" = true ]; then
    info "Stopping Stalwart Mail Server..."
    if [ "$PRODUCTION" = true ]; then
        docker-compose -f ../docker-compose.mail.ssl.yml down
    else
        docker-compose -f ../docker-compose.mail.yml down
    fi
    success "Stalwart Mail Server stopped."
    exit 0
fi

# Export environment
export MAIL_DOMAIN="$DOMAIN"

info "============================================"
info "Stalwart Mail Server Setup"
info "============================================"
info "Domain: $DOMAIN"
info "Mode: $([ "$PRODUCTION" = true ] && echo 'Production (HTTPS)' || echo 'Development')"
info "============================================"

# Create data directories
info "Creating data directories..."
mkdir -p ./data ./certs

# Update config with domain
info "Updating configuration..."
sed -i.bak "s/hostname = \"mail\.lurus\.local\"/hostname = \"mail.$DOMAIN\"/" ./config.toml

if [ "$PRODUCTION" = true ]; then
    warn "Production mode requires proper DNS configuration:"
    echo ""
    echo -e "${YELLOW}  Required DNS Records:${NC}"
    echo "  ----------------------"
    echo "  A     mail.$DOMAIN      -> <YOUR_SERVER_IP>"
    echo "  MX    $DOMAIN           -> mail.$DOMAIN (priority 10)"
    echo "  TXT   $DOMAIN           -> v=spf1 mx ~all"
    echo "  TXT   _dmarc.$DOMAIN    -> v=DMARC1; p=quarantine; rua=mailto:admin@$DOMAIN"
    echo ""

    read -p "Have you configured DNS records? (y/N) " confirm
    if [[ "$confirm" != "y" && "$confirm" != "Y" ]]; then
        warn "Please configure DNS records first, then run this script again."
        exit 1
    fi

    info "Starting Stalwart Mail Server (Production)..."
    docker-compose -f ../docker-compose.mail.ssl.yml up -d
else
    info "Starting Stalwart Mail Server (Development)..."
    docker-compose -f ../docker-compose.mail.yml up -d
fi

# Wait for startup
info "Waiting for Stalwart to start..."
sleep 10

# Health check
max_retries=30
retry_count=0
while [ $retry_count -lt $max_retries ]; do
    if curl -sf http://localhost:8080/healthz > /dev/null 2>&1; then
        success "Stalwart Mail Server is running!"
        break
    fi
    retry_count=$((retry_count + 1))
    echo -n "."
    sleep 2
done

if [ $retry_count -eq $max_retries ]; then
    error "Stalwart failed to start. Check logs with: docker logs stalwart-mail"
    exit 1
fi

echo ""
success "============================================"
success "Stalwart Mail Server is ready!"
success "============================================"
echo ""
echo -e "${CYAN}  Web Admin:    http://localhost:8080${NC}"
echo -e "${CYAN}  Admin Login:  admin / changeme${NC}"
echo ""
echo "  SMTP:         localhost:25 (MTA)"
echo "                localhost:587 (Submission)"
echo "                localhost:465 (SMTPS)"
echo ""
echo "  IMAP:         localhost:143 (STARTTLS)"
echo "                localhost:993 (IMAPS)"
echo ""
warn "IMPORTANT: Change the admin password immediately!"
echo ""
