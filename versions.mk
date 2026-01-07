# Copyright 2026 k8s-gpu-mcp-server contributors
# SPDX-License-Identifier: Apache-2.0

# Versions and tags
LIB_VERSION := 0.1.0
LIB_TAG ?= alpha

# Go toolchain
GOLANG_VERSION ?= 1.25

# Git information
GIT_COMMIT ?= $(shell git describe --match="" --dirty --long --always --abbrev=40 2> /dev/null || echo "")
GIT_TAG ?= $(shell git describe --tags --abbrev=0 2> /dev/null || echo "v0.0.0")

# Tool versions
GOLANGCI_LINT_VERSION ?= v1.61.0

