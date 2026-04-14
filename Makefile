

################################################################################################################
.PHONY: tidy
tidy:
	$(call _info,"Adding any missing module requirements...")
	@go mod tidy

################################################################################################################
.PHONY: upvendors
upvendors:
	$(call _info,"Updating packages...")
	@go get -u ./...

################################################################################################################
.PHONY: unit-test
unit-test:
	$(call _info,"Run unit tests...")
	@ GOFLAGS=-ldflags=-extldflags=-Wl,-ld_classic go test -race ./internal/... \
		&& go test -race ./pkg/...

################################################################################################################

.PHONY: lint
lint:
	$(call _info,"Run linters...")
	golangci-lint run -v --timeout 3m0s ./...

################################################################################################################
.PHONY: genmocks
genmocks:
	$(call _info,"Generating mocks...")
	@find . -name "mock.go" -type f -print| xargs -n 1 go generate

################################################################################################################
.PHONY: genmocks
gensqlc:
	$(call _info,"Generating sqlc...")
	@docker run --rm -v $(pwd)/sqlc:/src -w /src sqlc/sqlc generate

################################################################################################################
.PHONY: build
build:
	$(call _info,"Build with docker...")
	docker run --init -it --rm --env-file .env -v ${PWD}:/app -w /app  golang:1.25 go build -o palisade /app/cmd/cli/main.go

################################################################################################################
.PHONY: build-with-cache
build-with-cache:
	$(call _info,"Build with docker (Go module + build cache)...")
	@mkdir -p ${PWD}/.cache/go-mod ${PWD}/.cache/go-build
	docker run --init -it --rm --env-file .env \
		-v ${PWD}:/app \
		-v ${PWD}/.cache/go-mod:/go/pkg/mod \
		-v ${PWD}/.cache/go-build:/root/.cache/go-build \
		-w /app golang:1.25 go build -o palisade /app/cmd/cli/main.go

################################################################################################################