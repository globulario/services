.PHONY: specgen

specgen:
	./tools/specgen/specgen.sh \
	  /home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin \
	  ./generated

# --- Package generation defaults ---
PKGGEN_BIN_DIR ?= /home/dave/Documents/github.com/globulario/services/golang/tools/stage/linux-amd64/usr/local/bin
PKGGEN_GEN_ROOT ?= $(CURDIR)/generated
PKGGEN_OUT ?= $(CURDIR)/generated/packages
PKGGEN_VERSION ?= 0.0.1
PKGGEN_PUBLISHER ?= core@globular.io
PKGGEN_PLATFORM ?= $(shell go env GOOS)_$(shell go env GOARCH)
PKGGEN_GLOBULAR ?= globular

.PHONY: pkggen
pkggen: specgen
	./tools/pkggen/pkggen.sh \
	  --globular "$(PKGGEN_GLOBULAR)" \
	  --bin-dir "$(PKGGEN_BIN_DIR)" \
	  --gen-root "$(PKGGEN_GEN_ROOT)" \
	  --out "$(PKGGEN_OUT)" \
	  --version "$(PKGGEN_VERSION)" \
	  --publisher "$(PKGGEN_PUBLISHER)" \
	  --platform "$(PKGGEN_PLATFORM)"
