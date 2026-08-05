# glibc-floor.Dockerfile — the toolchain image release binaries are compiled in.
#
# WHY THIS EXISTS
# ---------------
# Go links cgo binaries against the BUILD host's glibc, and the versioned symbols
# it picks up become hard RUNTIME requirements. A developer or CI runner on a
# modern distro therefore silently raises the platform requirement of the whole
# release without changing a line of code.
#
# That is not hypothetical: releases 1.2.288 and 1.2.295 both shipped file_server
# requiring GLIBC_2.38 (pulled in via github.com/chai2010/webp -> libm
# fmod/fmodf@GLIBC_2.38) while docs/operators/building-from-source.md promises
# Ubuntu 22.04 (glibc 2.35). On that platform the binary does not start at all,
# and Day-0 aborts at "Workload Services".
#
# CGO_ENABLED=0 is NOT an escape hatch here: webp is a cgo binding, and
# file_server does not compile without it.
#
# So: pin the compiler to the floor. Ubuntu 22.04 IS the floor (glibc 2.35), so a
# binary produced here cannot require anything newer. This makes the support
# promise true by construction; scripts/check-glibc-floor.sh then only has to
# prove it stayed true.
#
# Keep this image's base in agreement with MAX_GLIBC in check-glibc-floor.sh and
# with the oldest platform named in docs/operators/building-from-source.md. If
# the floor moves, all three move together.
FROM ubuntu:22.04

# Matches the toolchain in golang/go.mod. Passed by build-local-release.sh from
# `go env GOVERSION` so the container and the host compile the same language
# version — only the libc differs, which is the entire point.
ARG GO_VERSION=1.25.0

# Build-time headers for the cgo services. These are NOT runtime dependencies —
# they are what the compiler needs to see, and a missing one fails the build
# loudly (`fatal error: sql.h: No such file or directory`) rather than silently
# producing a different binary, which is the behaviour we want.
#   unixodbc-dev — github.com/alexbrainman/odbc, used by sql_server
# (github.com/chai2010/webp vendors libwebp's C sources, so file_server needs no
# extra package here — only a C compiler.)
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates curl gcc g++ libc6-dev pkg-config git \
      unixodbc-dev \
 && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tgz \
 && tar -C /usr/local -xzf /tmp/go.tgz \
 && rm /tmp/go.tgz

# GOTOOLCHAIN=local: never let a go.mod toolchain directive silently download a
# different compiler inside the pinned image — that would reintroduce exactly the
# "the build host decided the requirements" problem this image exists to remove.
ENV PATH=/usr/local/go/bin:$PATH \
    GOTOOLCHAIN=local
