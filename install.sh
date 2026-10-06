#!/usr/bin/env sh
# Solar Chicken — first install (or update) with Docker Compose.
set -e
cd "$(dirname "$0")"

command -v docker >/dev/null 2>&1 || { echo "Docker is required: https://docs.docker.com/engine/install/"; exit 1; }
docker compose version >/dev/null 2>&1 || { echo "Docker Compose v2 is required (docker compose …)"; exit 1; }

rand() { openssl rand -hex 32 2>/dev/null || od -An -tx1 -N32 /dev/urandom | tr -d ' \n'; }

if [ ! -f .env ]; then
  cp .env.example .env
  for var in DB_PASSWORD JWT_SECRET SECRET_KEY; do
    value=$(rand)
    sed -i.bak "s/^$var=.*/$var=$value/" .env
  done
  rm -f .env.bak
  chmod 600 .env
  echo "✓ .env created with fresh secrets. Keep it safe: SECRET_KEY decrypts your Omlet key."
else
  echo "✓ Using the existing .env"
fi

# Prebuilt images when available, otherwise a local build.
docker compose pull --ignore-pull-failures >/dev/null 2>&1 || true
docker compose up -d

PORT=$(sed -n 's/^SOLAR_CHICKEN_PORT=//p' .env); PORT=${PORT:-3000}
HOST=$( (hostname -I 2>/dev/null || true) | awk '{print $1}'); HOST=${HOST:-localhost}
echo ""
echo "🐔 Solar Chicken is starting: http://$HOST:$PORT"
echo "   Create your account, then follow the setup (coop location, Omlet API key, doors)."
