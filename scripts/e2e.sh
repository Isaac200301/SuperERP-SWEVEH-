#!/bin/sh
# Recorrido en navegador real (Playwright) contra el api sirviendo la SPA en el mismo origen, como se despliega.
# Lo usan `make e2e` y el job e2e del CI. Necesita la base migrada y las variables de .env.example
# (DATABASE_URL, etc.). Solo APP_ENV=local: el código de entrada sale en el log del api (OTP_SENDER=consola).
set -eu
PORT="${E2E_PORT:-8090}"
# Playwright vacía OUT al empezar: el binario, el log y las fotos van aparte, en WORK.
OUT="${SWEVEH_E2E_OUT:-$(pwd)/.cache/e2e}"
WORK="${E2E_WORK:-$(pwd)/.cache/e2e-api}"
LOG="$WORK/api.log"
mkdir -p "$OUT" "$WORK"

npm --prefix web run build
go build -o "$WORK/api" ./cmd/api

APP_ENV=local OTP_SENDER=consola HTTP_ADDR=":$PORT" WEB_DIR="$(pwd)/web/dist" STORAGE_DIR="$WORK/fotos" \
  APP_ORIGINS="http://localhost:$PORT" "$WORK/api" >"$LOG" 2>&1 &
API_PID=$!
trap 'kill "$API_PID" 2>/dev/null || true' EXIT INT TERM

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

SWEVEH_WEB_URL="http://localhost:$PORT" SWEVEH_API_LOG="$LOG" SWEVEH_E2E_OUT="$OUT" npm --prefix web run e2e
