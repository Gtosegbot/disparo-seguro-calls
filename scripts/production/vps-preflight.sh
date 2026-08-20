#!/usr/bin/env bash
# ==============================================================================
# DS Voice — Remote VPS Preflight & Bring-Up Checker (Fase 17)
# ==============================================================================
set -euo pipefail

echo "=============================================================================="
echo " DS VOICE — REMOTE VPS PREFLIGHT AUDIT"
echo "=============================================================================="

# 1. OS check
OS_NAME=$(uname -s)
if [ "$OS_NAME" != "Linux" ]; then
    echo "[FAIL] Operating System: Linux is required. Got $OS_NAME."
    exit 1
else
    echo "[PASS] Operating System: Linux detected."
fi

# 2. Check essential command dependencies
for cmd in docker git curl jq openssl; do
    if ! command -v "$cmd" &> /dev/null; then
        echo "[FAIL] Dependency: '$cmd' is not installed."
        exit 1
    else
        echo "[PASS] Dependency: '$cmd' found."
    fi
done

# 3. Check Docker and Compose version
if ! docker info &> /dev/null; then
    echo "[FAIL] Docker daemon is not running."
    exit 1
else
    echo "[PASS] Docker daemon is healthy."
fi

COMPOSE_VER=$(docker compose version --short 2>/dev/null || echo "legacy")
if [ "$COMPOSE_VER" = "legacy" ]; then
    echo "[WARN] Docker Compose: Using legacy docker-compose parser."
else
    echo "[PASS] Docker Compose version: $COMPOSE_VER"
fi

# 4. Check network DNS and outbound connectivity to providers
echo "Testing outbound connectivity to real providers..."

# Test Google DNS
if ! curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 https://www.google.com &> /dev/null; then
    echo "[FAIL] DNS: Outbound HTTP/HTTPS network connectivity check failed."
    exit 1
else
    echo "[PASS] DNS: Outbound HTTP/HTTPS is functional."
fi

# Test Gemini Live WS endpoint availability
GEMINI_TEST=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 https://generativelanguage.googleapis.com || echo "000")
if [ "$GEMINI_TEST" = "000" ]; then
    echo "[WARN] Network: Gemini API endpoint is unreachable."
else
    echo "[PASS] Network: Gemini API endpoint connectivity checked (status $GEMINI_TEST)."
fi

# 5. Check environment variables
if [ ! -f .env ]; then
    echo "[FAIL] Deployment environment file (.env) is missing."
    exit 1
fi

REQUIRED_ENV=(
    "WACALLS_API_KEY"
    "WACALLS_XAI_KEY"
    "WACALLS_GEMINI_KEY"
    "DATABASE_URL"
    "REDIS_URL"
)

ENV_ERR=0
for var in "${REQUIRED_ENV[@]}"; do
    VAL=$(grep -E "^$var=" .env | cut -d'=' -f2- || true)
    if [ -z "$VAL" ]; then
        echo "[FAIL] Variable: $var is empty or missing in .env."
        ENV_ERR=1
    else
        echo "[PASS] Variable: $var is configured."
    fi
done

if [ "$ENV_ERR" -ne 0 ]; then
    echo "------------------------------------------------------------------------------"
    echo "[FAIL] Preflight failed due to missing configuration."
    exit 1
fi

echo "------------------------------------------------------------------------------"
echo "[SUCCESS] VPS Preflight audit completed. READY FOR DEPLOY!"
