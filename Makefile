# Go ставится в ~/.local/go, добавляем его в PATH для целей make
export PATH := $(HOME)/.local/go/bin:$(PATH)

.PHONY: dev dev-api dev-web test build web clean

# Go API на :8080 + Vite dev server на :5173 (проксирует /api)
dev:
	$(MAKE) -j2 dev-api dev-web

dev-api:
	go run ./cmd/cs2stats

dev-web:
	cd web && npm run dev

test:
	go vet ./...
	go test ./...
	cd web && npx tsc -b --noEmit

web:
	cd web && npm run build

# Бинарь со встроенным фронтом
build: web
	CGO_ENABLED=0 go build -tags embedweb -o cs2stats ./cmd/cs2stats

clean:
	rm -rf cs2stats internal/webui/dist
