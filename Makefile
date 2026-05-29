BINARY := tempest-mcp
INSTALL_DIR := $(HOME)/.local/bin
CONFIG_DIR := $(HOME)/.config/tempest-mcp

.PHONY: all build install deps run list-ports clean config

all: build

## deps: download Go module dependencies
deps:
	go mod tidy

## build: compile the binary to ./tempest-mcp
build: deps
	go build -o $(BINARY) ./cmd/tempest-mcp

## install: build and install to ~/.local/bin (add to PATH if needed)
install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"
	@echo "Make sure $(INSTALL_DIR) is in your PATH."

## config: create default config file if it doesn't exist
config:
	mkdir -p $(CONFIG_DIR)
	@if [ ! -f $(CONFIG_DIR)/config.yaml ]; then \
		echo 'midi:' > $(CONFIG_DIR)/config.yaml; \
		echo '  device_name: "Tempest"' >> $(CONFIG_DIR)/config.yaml; \
		echo '  channel: 10' >> $(CONFIG_DIR)/config.yaml; \
		echo '  clock_source: internal' >> $(CONFIG_DIR)/config.yaml; \
		echo 'library:' >> $(CONFIG_DIR)/config.yaml; \
		echo '  path: $(HOME)/Tempest' >> $(CONFIG_DIR)/config.yaml; \
		echo '  index_path: $(CONFIG_DIR)/library.json' >> $(CONFIG_DIR)/config.yaml; \
		echo '  auto_reindex: true' >> $(CONFIG_DIR)/config.yaml; \
		echo 'sysex:' >> $(CONFIG_DIR)/config.yaml; \
		echo '  inter_message_delay_ms: 1000' >> $(CONFIG_DIR)/config.yaml; \
		echo '  capture_dir: $(CONFIG_DIR)/captures' >> $(CONFIG_DIR)/config.yaml; \
		echo 'log:' >> $(CONFIG_DIR)/config.yaml; \
		echo '  level: info' >> $(CONFIG_DIR)/config.yaml; \
		echo '  midi_trace: false' >> $(CONFIG_DIR)/config.yaml; \
		echo "Created $(CONFIG_DIR)/config.yaml"; \
	else \
		echo "Config already exists at $(CONFIG_DIR)/config.yaml"; \
	fi

## list-ports: show available MIDI ports (useful to find the Tempest port name)
list-ports: build
	./$(BINARY) --list-ports

## run: run directly (for testing — Claude Desktop will manage this in production)
run: build
	./$(BINARY) --debug --device Tempest

## claude-config: print the claude_desktop_config.json snippet to add
claude-config: install
	@BINARY_PATH=$(INSTALL_DIR)/$(BINARY); \
	echo ''; \
	echo 'Add this to ~/Library/Application Support/Claude/claude_desktop_config.json:'; \
	echo ''; \
	echo '{'; \
	echo '  "mcpServers": {'; \
	echo '    "tempest": {'; \
	echo '      "command": "'$$BINARY_PATH'",'; \
	echo '      "args": ["--device", "Tempest"]'; \
	echo '    }'; \
	echo '  }'; \
	echo '}'; \
	echo ''

## clean: remove build artifacts
clean:
	rm -f $(BINARY)
