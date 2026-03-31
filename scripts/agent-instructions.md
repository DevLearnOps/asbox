# Sandbox Environment

You are running inside an isolated sandbox container. These are permanent constraints of this environment.

## System Access

- You are the `sandbox` user. There is NO `sudo` or root access.
- Do not attempt to install system packages via `apt-get` — it will fail.

## Container Runtime

- The container runtime is **rootless Podman**, aliased as `docker`.
- `docker compose` and `docker-compose` are both available.
- Use standard `docker` / `docker compose` commands — they work transparently via Podman.

## Playwright

- **chromium** and **webkit** browsers are supported in this sandbox.
- Chromium is used for desktop browser tests. WebKit is used for mobile device emulation (iPhone, iPad, etc.).
- System libraries required by both browsers are **pre-installed**.
- To install the Playwright browser binaries, run: `npx playwright install chromium webkit`
- **NEVER** use `--with-deps` flag — it requires root access which is not available.

## Running End-to-End Tests

When a project has a docker-compose stack and Playwright e2e tests:

1. Start the application stack first: `docker compose up -d` (wait for all services to be healthy)
2. Install the browser if needed: `npx playwright install chromium`
3. Run the tests: `npx playwright test`

## Git

- `git push` is disabled in this sandbox for safety. All other git operations work normally.
