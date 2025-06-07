# Makefile

FLUX_VERSION ?= latest
FLUX_INSTALL_DIR := pkg/flux/installer/manifests
FLUX_OUTPUT_FILE := $(FLUX_INSTALL_DIR)/gotk-components.yaml
FLUX_NAMESPACE := flux-system

.PHONY: flux-install clean

## flux-install: Generate Flux install manifests into /hack/flux-install
flux-install:
	@echo "Generating Flux install manifests into $(FLUX_INSTALL_DIR)..."
	@mkdir -p $(FLUX_INSTALL_DIR)
	flux install \
		--export \
		--namespace=$(FLUX_NAMESPACE) \
		> $(FLUX_OUTPUT_FILE)
	@echo "Flux manifests generated at $(FLUX_OUTPUT_FILE)"

## clean: Remove generated manifests
clean:
	@echo "Cleaning up generated manifests..."
	rm -rf $(FLUX_INSTALL_DIR)
