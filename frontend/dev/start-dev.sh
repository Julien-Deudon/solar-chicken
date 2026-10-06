#!/usr/bin/env bash
# Interface en local avec une fausse API (aucun serveur, aucune vraie porte).
# Connexion : n'importe quel e-mail, mot de passe « secret ».
# MOCK_FRESH=1 bash dev/start-dev.sh : premier démarrage (création du compte puis du poulailler).
set -euo pipefail
cd "$(dirname "$0")/.."
MOCK_PORT=${MOCK_PORT:-8787} node dev/mock-api.js &
MOCK_PID=$!
trap 'kill $MOCK_PID 2>/dev/null || true' EXIT INT TERM
API_INTERNAL_URL=http://localhost:${MOCK_PORT:-8787} npx next dev -p ${PORT:-3000}
