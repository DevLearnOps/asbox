# Manual Smoke Test Checklist

Run after each story completion or significant change. Requires a valid API key for at least one agent.

## Prerequisites

- [ ] Docker is running
- [ ] At least one agent API key is available (ANTHROPIC_API_KEY, GEMINI_API_KEY, or OPENAI_API_KEY)
- [ ] A test project directory exists with source code

---

## 1. Single Sandbox — Happy Path

- [ ] `asbox build` completes without error
- [ ] `asbox run` launches sandbox and drops into agent session
- [ ] Agent can read mounted project files
- [ ] Agent can execute git commands (add, commit, log)
- [ ] Agent can install packages via package manager (npm/pip/go)
- [ ] Agent can access the internet (curl a URL)
- [ ] Ctrl+C cleanly exits the sandbox
- [ ] `docker ps` confirms no orphaned containers after exit

## 2. Isolation Boundaries

- [ ] `git push` inside sandbox returns "Authentication failed" (not sandbox-aware message)
- [ ] Agent cannot see host `~/.ssh` or `~/.aws` contents
- [ ] Only explicitly mounted directories are visible in `/workspace`
- [ ] Secrets declared in config are available as env vars inside sandbox
- [ ] Undeclared host env vars are NOT visible inside sandbox

## 3. Concurrent Sandboxes

- [ ] Two sandboxes for **different** projects can run simultaneously
- [ ] Two sandboxes for the **same** project — verify behavior (expected: name collision error; track until fixed)

## 4. Image Caching

- [ ] Second `asbox build` with no config changes skips rebuild (cache hit)
- [ ] Changing a config value (e.g., adding a package) triggers rebuild
- [ ] `asbox run` auto-builds if image doesn't exist

## 5. Mount Persistence

- [ ] Files created by agent in mounted project dir persist after sandbox exit
- [ ] Git commits made inside sandbox are visible from host after exit
- [ ] With `auto_isolate_deps: true`, `node_modules` uses named volume (not host dir)
- [ ] Named volume persists across sandbox restarts (no re-install needed)

## 6. Multi-Agent Support

- [ ] `asbox run` launches default agent from config
- [ ] `asbox run --agent claude` overrides default agent
- [ ] `asbox run --agent gemini` launches Gemini CLI
- [ ] `asbox run --agent codex` launches Codex CLI
- [ ] Invalid agent name produces clear error with exit code 1

## 7. Host Agent Config

- [ ] With `host_agent_config: true`, agent can read OAuth tokens from host config
- [ ] With `host_agent_config: false`, no host config is mounted

## 8. MCP Integration

- [ ] With `mcp_servers: [playwright]`, Playwright is available inside sandbox
- [ ] MCP manifest merge: project `.mcp.json` takes precedence over build-time manifest

## 9. BMAD Repos (if configured)

- [ ] Configured repos are mounted at `/workspace/repos/<name>`
- [ ] Agent instruction file references repo paths
- [ ] Repos with basename collision produce clear error

## 10. Configuration Init

- [ ] `asbox init` generates starter config without requiring Docker
- [ ] Generated config is valid YAML and can be used with `asbox build`

---

## Post-Test Cleanup

- [ ] `docker ps -a` — no orphaned asbox containers
- [ ] `docker network ls` — no orphaned asbox networks
- [ ] `docker volume ls` — only expected named volumes remain
