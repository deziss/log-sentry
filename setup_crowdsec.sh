#!/bin/bash
set -e

# 1. Start CrowdSec
echo "Starting CrowdSec..."
docker compose up -d crowdsec

# 2. Wait for LAPI to be ready
echo "Waiting for CrowdSec LAPI..."
until docker compose exec -T crowdsec cscli lapi status > /dev/null 2>&1; do
    echo "Waiting for LAPI..."
    sleep 2
done

# 3. Generate API Key
echo "Generating API Key for log-sentry..."
# Clean up old bouncer if exists
docker compose exec -T crowdsec cscli bouncers delete log-sentry > /dev/null 2>&1 || true

# Add new bouncer and capture key
API_KEY=$(docker compose exec -T crowdsec cscli bouncers add log-sentry -o raw)

if [ -z "$API_KEY" ]; then
    echo "Failed to generate API Key"
    exit 1
fi

echo "Generated API Key: $API_KEY"

# 4. Update docker-compose.yml or .env?
# Since we can't easily edit docker-compose.yml responsibly with sed (formatting risks), 
# we'll ask user to set it or export it.

echo ""
echo "=================================================="
echo "ACTION REQUIRED: Update docker-compose.yml"
echo "=================================================="
echo "Replace 'your_key_here' with:"
echo "CROWDSEC_API_KEY=$API_KEY"
echo "ENABLE_CROWDSEC=true"
echo "=================================================="
