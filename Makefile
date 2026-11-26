BINARY=kettle
PROTO_DIR=./api/shim
OUT_DIR=./api/shim
PROTOC_GEN_TTRPC=$(shell which protoc-gen-ttrpc)

# hack/ is vendored containerd client code that does not build on its own.
PACKAGES=$(shell go list ./... | grep -v -e /hack -e /integration)
# Packages with tests that need root, run with -test.root (as in containerd).
TEST_REQUIRES_ROOT_PACKAGES=$(shell go list ./... | grep -v -e /hack -e /integration)
TESTFLAGS ?=
GO_TEST ?= go test

build-all:
	go build -o $(BINARY)

test: ## run tests, except integration tests and tests that require root
	$(GO_TEST) $(TESTFLAGS) $(PACKAGES)

root-test: ## run tests, except integration tests
	sudo -E env "PATH=$$PATH" $(GO_TEST) $(TESTFLAGS) $(TEST_REQUIRES_ROOT_PACKAGES) -test.root

integration: ## run integration tests (needs root and network)
	sudo -E env "PATH=$$PATH" $(GO_TEST) -v $(TESTFLAGS) -count=1 ./integration -test.root

# Generate Go code from proto files for ttrpc
generate-proto:
	@mkdir -p $(OUT_DIR)
	@find $(PROTO_DIR) -name "*.proto" | while read proto_file; do \
		protoc \
			-I=$(PROTO_DIR) \
			--go-ttrpc_out=$(OUT_DIR) \
      --go_out=$(OUT_DIR) \
			$$proto_file; \
	done

.PHONY: check-tools generate-proto build clean test root-test integration

