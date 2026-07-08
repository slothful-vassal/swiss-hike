PLUGIN_ID := swiss-hike
DIST_DIR := dist/$(PLUGIN_ID)
PLUGIN_DIST_DIR := plugin_dist

.PHONY: build manifest package clean

build: manifest
	tinygo build -target=wasi -scheduler=none -no-debug -o $(DIST_DIR)/plugin.wasm .

manifest:
	mkdir -p $(DIST_DIR)
	go run github.com/open-wanderer/wanderer/plugins/sdk/cmd/manifestcheck > $(DIST_DIR)/plugin.json
	cp assets/icon.svg $(DIST_DIR)/icon.svg
	cp assets/icon_dark.svg $(DIST_DIR)/icon_dark.svg

package: build
	rm -rf $(PLUGIN_DIST_DIR)
	mkdir -p $(PLUGIN_DIST_DIR)
	tar -C dist -czf $(PLUGIN_DIST_DIR)/wanderer-plugin-$(PLUGIN_ID).tar.gz $(PLUGIN_ID)
	cd $(PLUGIN_DIST_DIR) && sha256sum *.tar.gz > SHA256SUMS

clean:
	rm -rf dist $(PLUGIN_DIST_DIR)
