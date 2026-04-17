# Future Work

## Change container name rendering to allow multiple agent sessions per project

At the moment we don't have the option to run multiple sandboxes against the same project because of sandbox container name conflict. We could either:
- change the logic to include a random seed to the sandbox name
- allow users to override the name via cli options
- or maybe both, and only seed randomly if there is name conflict


## Pre-install DevOps tools for infrastructure changes validation and investigation

This feature is about having tools in the sandbox that are useful for IaC and pipeline development and validation.
Some jobs are better suited for direct access via cli programs in the sandbox (e.g. validating a terraform module, or checking for vulnerability with a trivy local scan).
Some other job would be better suited to have an MCP support, like accessing to our live observability platform with a read-only role. Or checking current cluster state.


## Shorthand CLI arguments to improve user experience

Simple targeted story to allow users some quick access to running different agents than the default.
- `asbox run -a <agent>`
- agent override should also be available with no argument. We accept the overload in favor of saving the developer the friction of using a flag (`asbox run codex`).


## Deferred from: code review of 11-5-pinned-build-dependencies (2026-04-17)

- No checksum verification on Docker Compose binary download — `embed/Dockerfile.tmpl` fetches Docker Compose via curl without SHA256 integrity check. Pre-existing pattern, not introduced by version pinning.
- Build-time `npx playwright install` browser versions determined by transitive deps — browser builds fetched at build time are controlled by `@playwright/mcp`'s dependency tree, not explicitly pinned. Pre-existing pattern.


## BMAD Workflow Repository Status Management

When working on changes that affect multiple projects it can be painful to manage git branching and stale changes across all of them. The agent is fully capable of stashing changes or move to a feature branch across all of them autonomously but:
- it needs specific and unequivocal instructions. AGENTS.md files should contain instructions of how to manage branching to resume work.
- sandboxed agent is unable to fetch upstream state, need to explore options to perform this operation on `asbox run` or create a specialised command like `asbox fetch` to ensure local repos have all information from remote even on non-current branches.
