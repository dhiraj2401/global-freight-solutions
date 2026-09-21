TAILWIND ?= ./bin/tailwindcss
HTMX_VERSION := 4.0.0

.PHONY: dev css css-watch tailwind htmx build test vet migrate docker

dev: ## Run the site locally on :8080
	go run ./cmd/server

css: $(TAILWIND) ## Compile static/css/app.css (commit the result)
	$(TAILWIND) -i static/css/tailwind.css -o static/css/app.css --minify

css-watch: $(TAILWIND)
	$(TAILWIND) -i static/css/tailwind.css -o static/css/app.css --watch

$(TAILWIND):
	./scripts/install-tailwind.sh

tailwind: $(TAILWIND) ## Download the Tailwind v4 standalone CLI

htmx: ## Re-download htmx
	curl -fsSL -o static/js/htmx.min.js https://cdn.jsdelivr.net/npm/htmx.org@$(HTMX_VERSION)/dist/htmx.min.js

build: ## Build a static binary
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/server ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

migrate: ## Apply database migrations
	go run ./cmd/migrate

docker:
	docker build -t gfs-website .
