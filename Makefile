GOOS = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
BUILD_DIR = dist/${GOOS}_${GOARCH}
GENERATED_CONF := pkg/config/conf.gen.go
GENERATED_SETUP_ARTIFACTS := $(GENERATED_CONF) config_schema.json

ifeq ($(GOOS),windows)
OUTPUT_PATH = ${BUILD_DIR}/baton-gitlab.exe
else
OUTPUT_PATH = ${BUILD_DIR}/baton-gitlab
endif

# Set the build tag conditionally based on ENABLE_LAMBDA
ifdef BATON_LAMBDA_SUPPORT
	BUILD_TAGS=-tags baton_lambda_support
else
	BUILD_TAGS=
endif

.PHONY: build generate-setup-artifacts verify-setup-artifacts
build: $(GENERATED_CONF)
	go build ${BUILD_TAGS} -o ${OUTPUT_PATH} ./cmd/baton-gitlab
    
$(GENERATED_CONF): pkg/config/config.go go.mod
	@echo "Generating $(GENERATED_CONF)..."
	go generate ./pkg/config
    
generate: $(GENERATED_CONF)

generate-setup-artifacts:
	@echo "Generating $(GENERATED_CONF)..."
	go generate ./pkg/config
	@echo "Building connector for config schema generation..."
	go build ${BUILD_TAGS} -o ${OUTPUT_PATH} ./cmd/baton-gitlab
	@echo "Generating config_schema.json..."
	${OUTPUT_PATH} config > config_schema.json

verify-setup-artifacts: generate-setup-artifacts
	@stale_paths="$$(git diff --name-only -- $(GENERATED_SETUP_ARTIFACTS))"; \
	if [ -n "$$stale_paths" ]; then \
		echo "Setup artifacts are stale:"; \
		printf '%s\n' "$$stale_paths"; \
		git diff --exit-code -- $(GENERATED_SETUP_ARTIFACTS); \
	fi

.PHONY: update-deps
update-deps:
	go get -d -u ./...
	go mod tidy -v
	go mod vendor

.PHONY: add-deps
add-dep:
	go mod tidy -v
	go mod vendor
	
.PHONY: lint
lint:
	golangci-lint run
