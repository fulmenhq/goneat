#!/usr/bin/env bash
# Authoritative goneat release matrix. Third-party tool manifests are independent.
# shellcheck disable=SC2034 # Consumed by build-all.sh and package-artifacts.sh.
RELEASE_TARGETS=(
	"linux/amd64"
	"linux/arm64"
	"darwin/arm64"
	"windows/amd64"
	"windows/arm64"
)
