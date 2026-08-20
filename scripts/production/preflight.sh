#!/usr/bin/env bash
# ==============================================================================
# DS Voice — Production Preflight Validation Script (Fase 15)
# ==============================================================================
set -euo pipefail

echo "=============================================================================="
echo " DS VOICE — VPS PRODUCTION PREFLIGHT"
echo "=============================================================================="

# 1. OS check
OS_NAME=$(uname -s)
if [ "$OS_NAME" != "Linux" ]; then
    echo "[FAIL] OS: Required Linux, got $OS_NAME"
    exit 1
else
    echo "[PASS] OS: Linux detected ($(uname -r))"
fi

# 2. Check required dependencies
for cmd in docker git curl openssl jq; do
    if ! command -v "$cmd" &> /dev/null; then
        echo "[FAIL] Required command '$cmd' is not installed."
        exit 1
    else
        echo "[PASS] Command '$cmd' found."
    fi
done

# 3. Check Docker daemon running
if ! docker info &> /dev/null; then
    echo "[FAIL] Docker daemon is not running."
    exit 1
else
    echo "[PASS] Docker daemon active."
fi

# 4. Check compose version
COMPOSE_VER=$(docker compose version --short 2>/dev/null || echo "legacy")
if [ "$COMPOSE_VER" = "legacy" ]; then
    echo "[WARN] Docker Compose: Using legacy docker-compose command."
else
    echo "[PASS] Docker Compose version: $COMPOSE_VER"
fi

# 5. Check variables from .env
if [ ! -f .env ]; then
    echo "[FAIL] .env file not found. Copy from .env.example first."
    exit 1
fi

REQUIRED_ENV=(
    "WACALLS_API_KEY"
    "WACALLS_XAI_KEY"
    "WACALLS_GEMINI_KEY"
)

ENV_ERR=0
for var in "${REQUIRED_ENV[@]}"; do
    VAL=$(grep -E "^$var=" .env | cut -d'=' -f2- || true)
    if [ -z "$VAL" ]; then
        echo "[FAIL] Environment Variable: $var is empty or missing in .env"
        ENV_ERR=1
    else
        echo "[PASS] Environment Variable: $var is configured."
    fi
done

if [ "$ENV_ERR" -ne 0 ]; then
    echo "------------------------------------------------------------------------------"
    echo "[FAIL] Preflight failed due to missing environment variables."
    exit 1
fi

# 6. Check free memory
FREE_MEM_KB=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
FREE_MEM_GB=$((FREE_MEM_KB / 1024 / 1024))
if [ "$FREE_MEM_GB" -lt 2 ]; then
    echo "[WARN] Free RAM: $FREE_MEM_GB GB available. Recommended minimum is 2GB."
else
    echo "[PASS] Free RAM: $FREE_MEM_GB GB available."
fi

# 7. Check free disk
FREE_DISK_GB=$(df -BG / | tail -1 | awk '{print $4}' | tr -d 'G')
if [ "$FREE_DISK_GB" -lt 5 ]; then
    echo "[WARN] Free Disk: $FREE_DISK_GB GB available. Recommended minimum is 5GB."
else
    echo "[PASS] Free Disk: $FREE_DISK_GB GB available."
fi

echo "------------------------------------------------------------------------------"
echo "[SUCCESS] VPS Preflight completed successfully!"
