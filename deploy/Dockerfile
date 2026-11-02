# Imagen única de SWEVEH: api, worker y migrate en una imagen sin root (docs/06 §12.8).
# El frontend compilado va dentro y lo sirve el api desde el mismo origen (ADR-18).
# Las imágenes base se fijan por versión; fijarlas por digest es la siguiente mejora (Snyk Container).

FROM node:26-bookworm AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY api/openapi.yaml /src/api/openapi.yaml
COPY web/ ./
RUN npm run build

FROM golang:1.27 AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY db ./db
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker \
 && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/api /out/worker /out/migrate /app/
COPY --from=web /src/web/dist /app/web
ENV WEB_DIR=/app/web HTTP_ADDR=:8080
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api"]
