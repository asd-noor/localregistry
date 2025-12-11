# CLI Bug Report

## Summary

This document lists bugs discovered during CLI testing of `local-registry` tool.

**Status:** All bugs have been fixed.

---

## BUG-001: Default Protocol is HTTPS Instead of HTTP

**Severity:** High  
**Component:** `internal/api/client.go` (lines 47-54)  
**Status:** FIXED

**Description:**  
The API client defaults to HTTPS when no protocol is specified and `Insecure` is false. For a local development registry, HTTP should be the default since most local registries run without TLS.

**Fix:**  
Changed `NewClient()` to default to HTTP. The `--insecure` flag now only controls TLS verification for HTTPS connections.

---

## BUG-002: Misleading `--insecure` Flag Description

**Severity:** Medium  
**Component:** `cmd/root.go` (line 113)  
**Status:** FIXED

**Description:**  
The `--insecure` flag was documented as "Skip TLS certificate verification" but it actually switched protocol from HTTPS to HTTP.

**Fix:**  
Updated flag description to "Skip TLS verification (for HTTPS registries)" and changed default protocol to HTTP.

---

## BUG-003: Inconsistent Argument Format Across Commands

**Severity:** Medium  
**Component:** Multiple command files  
**Status:** FIXED

**Description:**  
Some commands accepted `image:tag` format while others required separate `<repository> <tag>` arguments.

**Fix:**  
All commands now accept unified `<image>` format:
- `name` (implies latest tag)
- `name:tag`
- `name@sha256:digest`
- `repository/name:tag`

| Command | Old Format | New Format |
|---------|------------|------------|
| `tags` | `<repository>` | `<image>` (strips tag) |
| `manifest get` | `<repository> <reference>` | `<image>` |
| `manifest head` | `<repository> <reference>` | `<image>` |
| `manifest delete` | `<repository> <reference>` | `<image>` |
| `delete tag` | `<repository> <tag>` | `<image>` |
| `delete image` | `<repository> [tags...]` | `<image> [tags...]` |
| `delete repo` | `<repository>` | `<image>` (strips tag) |

---

## BUG-004: `tags` Command Doesn't Handle `image:tag` Format Gracefully

**Severity:** Low  
**Component:** `cmd/tags.go`  
**Status:** FIXED

**Description:**  
The `tags` command expected only a repository name but if the user provided `image:tag`, it failed with a confusing 404 error.

**Fix:**  
The `tags` command now strips the tag portion using `stripTag()` helper function.

---

## BUG-005: No Input Validation for Empty Image Name in `add` Command

**Severity:** Low  
**Component:** `cmd/add.go`  
**Status:** FIXED

**Description:**  
The `add` command didn't validate that the source image is non-empty before attempting the skopeo copy operation.

**Fix:**  
Added early validation with clear error message: "source image cannot be empty"

---

## BUG-006: Delete Container Name Flag Inconsistent Default

**Severity:** Low  
**Component:** `cmd/delete.go`, `cmd/server.go`, `cmd/tui.go`  
**Status:** FIXED

**Description:**  
The `--container` flag default was `"registry"` but the actual container name from config is `"local-registry"`.

**Fix:**  
Changed flag default to empty string. The code falls back to config value when flag is not explicitly set. Help text no longer shows misleading default.

---

## Testing Environment

- **Build Command:** `make build`
- **Binary:** `bin/local-registry`
- **Platform:** Linux
- **Go Version:** See `.go-version`

## Commands Tested (Post-Fix)

| Command | Status |
|---------|--------|
| `version` | OK |
| `config` | OK |
| `ping` | OK |
| `catalog` | OK |
| `tags` | OK |
| `add` | OK |
| `inspect` | OK |
| `copy` | OK |
| `manifest get` | OK |
| `manifest head` | OK |
| `manifest delete` | OK |
| `delete tag` | OK |
| `delete image` | OK |
| `delete repo` | OK |
| `server status` | OK |
| `server start` | OK |
| `server stop` | OK |
| `server restart` | OK |
| `server gc` | OK |
| `server info` | OK |
| `completion` | OK |
| `tui` | OK (requires TTY) |
