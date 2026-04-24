---
title: 'Bump Coding Agent Versions'
type: 'chore'
created: '2026-04-24'
status: 'done'
route: 'one-shot'
---

# Bump Coding Agent Versions

## Intent

**Problem:** `embed/Dockerfile.tmpl` pinned Gemini CLI below the current npm release, and the header still documented an older Codex CLI version than the install command used.

**Approach:** Update the Gemini CLI npm pin to the latest registry version and make the header's Codex CLI pin match the existing install command.

## Suggested Review Order

- Confirm documented agent pins match the intended package versions.
  [`Dockerfile.tmpl:7`](../../embed/Dockerfile.tmpl#L7)

- Confirm Gemini installs the npm registry version checked during the bump.
  [`Dockerfile.tmpl:293`](../../embed/Dockerfile.tmpl#L293)
