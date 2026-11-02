#!/bin/sh
# Cobertura de Go medida entre paquetes (las pruebas de punta a punta de internal/server cubren
# los casos de uso de todos los módulos). Excluye código generado (sqlcgen) y el ayudante de pruebas,
# con la justificación de R-11. Falla si dominio o aplicación quedan por debajo del umbral.
set -eu
THRESHOLD="${COVERAGE_MIN:-80}"
OUT="${COVERAGE_OUT:-.cache/cover.out}"
mkdir -p "$(dirname "$OUT")"

go test -race -count=1 -coverpkg=./internal/... -coverprofile="$OUT.raw" ./...
grep -v -E 'sqlcgen|internal/platform/testdb' "$OUT.raw" > "$OUT"

fail=0
for p in identity/domain identity/app customers/domain customers/app vehicles/domain vehicles/app \
         orders/domain orders/app tenancy/domain quotes/domain quotes/app inventory/domain inventory/app \
         catalog/domain catalog/app; do
  grep -E "^mode|sweveh/internal/$p/" "$OUT" > "$OUT.pkg"
  pct=$(go tool cover -func="$OUT.pkg" | tail -1 | awk '{print $3}' | tr -d '%')
  printf '%-20s %s%%\n' "$p" "$pct"
  if awk "BEGIN{exit !($pct < $THRESHOLD)}"; then fail=1; fi
done
printf 'total (sin generado) %s\n' "$(go tool cover -func="$OUT" | tail -1 | awk '{print $3}')"
rm -f "$OUT.pkg" "$OUT.raw"
if [ "$fail" -ne 0 ]; then
  echo "cobertura por debajo de ${THRESHOLD}% en dominio o aplicación (R-11)" >&2
  exit 1
fi
