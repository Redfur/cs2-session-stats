# 1. Сборка фронтенда (Vite кладёт результат в internal/webui/dist)
FROM node:22-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# 2. Сборка Go-бинаря со встроенным фронтендом
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/webui/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -tags embedweb -trimpath -ldflags="-s -w" -o /cs2stats ./cmd/cs2stats

# 3. Минимальный образ
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /cs2stats /cs2stats
ENV DATA_DIR=/data ADDR=:8080
EXPOSE 8080
ENTRYPOINT ["/cs2stats"]
