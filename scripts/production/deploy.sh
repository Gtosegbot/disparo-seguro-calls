#!/usr/bin/env bash
# ==============================================================================
# DS Voice — Production Deployment Script (Fase 15)
# ==============================================================================
set -euo pipefail

echo "=============================================================================="
echo " DS VOICE — DEPLOYING TO VPS PRODUCTION"
echo "=============================================================================="

# 1. Run Preflight
echo "Running VPS Preflight checks..."
bash ./scripts/production/preflight.sh

# 2. Pull latest changes
echo "Pulling latest main branch changes..."
git pull --ff-only

# 3. Verify Compose Configuration
echo "Verifying docker-compose config..."
docker compose config > /dev/null

# 4. Pull and Build Images
echo "Building docker images..."
docker compose build --pull

# 5. Bring up dependencies first
echo "Starting PostgreSQL and Redis..."
docker compose up -d db redis

# Wait for DB to be healthy
echo "Waiting for database connection to be ready..."
sleep 5

# 6. Start Application
echo "Starting wacalls application..."
docker compose up -d wacalls

# 7. Verify Health Checks
echo "Verifying application service health..."
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health || echo "000")
if [ "$HTTP_STATUS" != "200" ]; then
    echo "[FAIL] Liveness check returned status $HTTP_STATUS"
    exit 1
else
    echo "[PASS] /health check: OK (200)"
fi

HTTP_READY=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/ready || echo "000")
if [ "$HTTP_READY" != "200" ]; then
    echo "[FAIL] Readiness check returned status $HTTP_READY"
    exit 1
else
    echo "[PASS] /ready check: OK (200)"
fi

echo "=============================================================================="
echo "[SUCCESS] Deployment completed successfully!"
echo "=============================================================================="
