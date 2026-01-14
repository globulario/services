.PHONY: specgen pkggen all-packages

STAGE_BIN_DIR ?= /home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin
GEN_DIR ?= $(CURDIR)/generated
PKG_OUT ?= $(GEN_DIR)/packages
PKG_VERSION ?= 0.0.1
PKG_PUBLISHER ?= core@globular.io
PKG_PLATFORM ?= $(shell go env GOOS)_$(shell go env GOARCH)
GLOBULARCLI ?= $(CURDIR)/globularcli

specgen:
	./tools/specgen/specgen.sh "$(STAGE_BIN_DIR)" "$(GEN_DIR)"

pkggen: specgen
	./tools/pkggen/pkggen.sh \
	  --globular "$(GLOBULARCLI)" \
	  --bin-dir "$(STAGE_BIN_DIR)" \
	  --gen-root "$(GEN_DIR)" \
	  --out "$(PKG_OUT)" \
	  --version "$(PKG_VERSION)" \
	  --publisher "$(PKG_PUBLISHER)" \
	  --platform "$(PKG_PLATFORM)"

all-packages: pkggen
	@echo "Packages available in: $(PKG_OUT)"
