#!/bin/sh
# Recorrido en navegador real (Playwright) contra el api sirviendo la SPA en el mismo origen, como se despliega,
# y el worker (webhooks de WhatsApp y outbox) con el canal de prueba. Lo usan `make e2e` y el job e2e del CI.
# Necesita la base migrada y las variables de .env.example (DATABASE_URL, WORKER_DATABASE_URL). Solo APP_ENV=local:
# el código de entrada sale en el log del api (OTP_SENDER=consola).
set -eu
# Entrega del 27-nov (V-01): por defecto sin WhatsApp, como el piloto. E2E_WHATSAPP=true corre también whatsapp.spec.ts.
E2E_WHATSAPP="${E2E_WHATSAPP:-false}"
export E2E_WHATSAPP
PORT="${E2E_PORT:-8090}"
# Playwright vacía OUT al empezar: el binario, el log y las fotos van aparte, en WORK.
OUT="${SWEVEH_E2E_OUT:-$(pwd)/.cache/e2e}"
WORK="${E2E_WORK:-$(pwd)/.cache/e2e-api}"
LOG="$WORK/api.log"
mkdir -p "$OUT" "$WORK"

npm --prefix web run build
go build -o "$WORK/api" ./cmd/api
go build -o "$WORK/worker" ./cmd/worker

APP_ENV=local FEATURE_WHATSAPP="$E2E_WHATSAPP" OTP_SENDER=consola HTTP_ADDR=":$PORT" WEB_DIR="$(pwd)/web/dist" STORAGE_DIR="$WORK/fotos" \
  APP_ORIGINS="http://localhost:$PORT" "$WORK/api" >"$LOG" 2>&1 &
API_PID=$!
APP_ENV=local FEATURE_WHATSAPP="$E2E_WHATSAPP" WHATSAPP_PROVIDER=local STORAGE_DIR="$WORK/fotos" "$WORK/worker" >"$WORK/worker.log" 2>&1 &
WORKER_PID=$!
# Solo se apagan los procesos que este script levantó (por PID), nunca por nombre.
trap 'kill "$API_PID" "$WORKER_PID" 2>/dev/null || true' EXIT INT TERM

i=0
until curl -fsS "http://localhost:$PORT/healthz" >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -gt 60 ] || ! kill -0 "$API_PID" 2>/dev/null; then
    echo "el api no arrancó; últimas líneas del log:" >&2
    tail -n 40 "$LOG" >&2
    exit 1
  fi
  sleep 0.5
done

# Los argumentos van a Playwright (p. ej. `E2E_WHATSAPP=true sh scripts/e2e.sh whatsapp.spec.ts`).
SWEVEH_WEB_URL="http://localhost:$PORT" SWEVEH_API_LOG="$LOG" SWEVEH_E2E_OUT="$OUT" npm --prefix web run e2e -- "$@"
