BINARY := apple-panic
WEB_DIR := web
WASM_OUT := $(WEB_DIR)/apple-panic.wasm
GOROOT := $(shell go env GOROOT)
WASM_EXEC := $(firstword $(wildcard $(GOROOT)/lib/wasm/wasm_exec.js $(GOROOT)/misc/wasm/wasm_exec.js))

.PHONY: all desktop build web serve test clean

all: desktop web

desktop:
	go build -o $(BINARY) .

build: desktop

web:
	@mkdir -p $(WEB_DIR)
	@test -n "$(WASM_EXEC)" || (echo "Error: wasm_exec.js not found in $(GOROOT)" >&2; exit 1)
	GOOS=js GOARCH=wasm go build -o $(WASM_OUT) .
	cp "$(WASM_EXEC)" $(WEB_DIR)/wasm_exec.js
	@WASM_HASH=$$( (sha256sum $(WASM_OUT) 2>/dev/null || shasum -a 256 $(WASM_OUT)) | cut -c1-12 ); \
	JS_HASH=$$( (sha256sum $(WEB_DIR)/wasm_exec.js 2>/dev/null || shasum -a 256 $(WEB_DIR)/wasm_exec.js) | cut -c1-12 ); \
	sed -E \
		-e "s|wasm_exec\.js(\?v=[0-9a-f]+)?|wasm_exec.js?v=$$JS_HASH|g" \
		-e "s|apple-panic\.wasm(\?v=[0-9a-f]+)?|apple-panic.wasm?v=$$WASM_HASH|g" \
		$(WEB_DIR)/index.html > $(WEB_DIR)/index.html.tmp && \
	mv $(WEB_DIR)/index.html.tmp $(WEB_DIR)/index.html

serve: web
	python3 -m http.server 8080 -d $(WEB_DIR)

test:
	go test ./...

clean:
	rm -f $(BINARY) $(WASM_OUT) $(WEB_DIR)/wasm_exec.js
