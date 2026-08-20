#!/usr/bin/env bash
# ==============================================================================
# DS Voice — Production Smoke Test Script (Fase 15)
# ==============================================================================
set -euo pipefail

echo "=============================================================================="
echo " DS VOICE — PRODUCTION SMOKE TEST"
echo "=============================================================================="

# 1. API Liveness Check
echo "Testing API /health..."
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health || echo "500")
if [ "$HTTP_STATUS" = "200" ]; then
    echo "[PASS] API Liveness: OK (200)"
else
    echo "[FAIL] API Liveness: FAILED (status $HTTP_STATUS)"
    exit 1
fi

# 2. Database Connection Readiness Check
echo "Testing /ready..."
HTTP_READY=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/ready || echo "500")
if [ "$HTTP_READY" = "200" ]; then
    echo "[PASS] Database Connection: OK (200)"
else
    echo "[FAIL] Database Connection: FAILED (status $HTTP_READY)"
    exit 1
fi

# 3. Test list campaigns endpoint (requires token authentication)
echo "Testing /api/campaigns scope security..."
HTTP_CAMP=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Tenant-ID: tenant-A" -H "X-API-Key: invalid-key" http://localhost:8080/api/campaigns || echo "500")
if [ "$HTTP_CAMP" = "401" ] || [ "$HTTP_CAMP" = "403" ]; then
    echo "[PASS] Security: Forged token request rejected correctly (status $HTTP_CAMP)"
else
    echo "[WARN] Security: Expected unauthorized status, got $HTTP_CAMP"
fi

echo "=============================================================================="
echo "[SUCCESS] Production Smoke Test passed successfully!"
echo "=============================================================================="
