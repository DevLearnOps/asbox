# Future Work

> **Status (2026-04-17):** All items below have been folded into the PRD, architecture, and epic plan. New FRs FR60–FR66 and NFR16 in `planning-artifacts/prd.md`. New Epics 12–15 in `planning-artifacts/epics.md`, tracked in `sprint-status.yaml` as `backlog`. This document is retained as the capture log.

## Shorthand CLI arguments to improve user experience
_Covered by: FR60, FR61 → **Epic 12: CLI Ergonomics for Agent Override** (Story 12.1). Decision: both `-a` and positional agent supported; providing both forms errors with exit code 2._

Simple targeted story to allow users some quick access to running different agents than the default.
- `asbox run -a <agent>`
- agent override should also be available with no argument. We accept the overload in favor of saving the developer the friction of using a flag (`asbox run codex`).


## BMAD Workflow Repository Status Management
_Covered by: FR64, FR65 → **Epic 13: Multi-Repo State Management** (Stories 13.1, 13.2). Decision: branch guidance baked into the generated agent instructions (opinionated single convention); upstream sync is a `--fetch` flag on `asbox run` rather than a new top-level command._

When working on changes that affect multiple projects it can be painful to manage git branching and stale changes across all of them. The agent is fully capable of stashing changes or move to a feature branch across all of them autonomously but:
- it needs specific and unequivocal instructions. AGENTS.md files should contain instructions of how to manage branching to resume work.
- sandboxed agent is unable to fetch upstream state, need to explore options to perform this operation on `asbox run` or create a specialised command like `asbox fetch` to ensure local repos have all information from remote even on non-current branches.


## Installation of common devops tools for implementation validation
_Covered by: FR62, NFR16 → **Epic 14: Pre-Installed Validation & Exploration Toolchain** (Story 14.1)._

Common devops tools should be installed by default into the sandbox to allow the agent to self-validate its work. Example of such tools are: git, kubectl, helm, kustomize, yq, jq, opentofu, tflint, kubeconform, kube-linter, trivy, flux, sops.
Tools don't need credentials or authentication information to work in the sandbox. They will be primarily used to validate work like formatting terraform code or test rendering of helm charts.
Installed versions should be pinned to specific current latest versions.
For tools that need to download artefacts like trivy databases we should ensure the sandbox user has appropriate permissions to download them into their default location (typically user home dir dot-folder).


## Installation of agent tools for repository exploration
_Covered by: FR63, NFR16 → **Epic 14: Pre-Installed Validation & Exploration Toolchain** (Story 14.2). Note: `git ls-files` already satisfied by existing git install (FR23), so it is called out in the agent instructions rather than being a separate install._

We want to make sure the sandbox has the necessary CLI tools for code repository exploration and file searches to aid completion of coding tasks.
Installed versions should be pinned to specific current latest versions.
- ripgrep is the standard fast recursive code search tool and respects .gitignore.
- fd is a fast parallel file finder and is much better than find for agent-style traversal.
- git ls-files is often the best starting point for repo tree construction because it avoids a lot of junk automatically.
- ast-grep helps when plain text search is too noisy and the agent needs structural matches.
- universal-ctags gives a broad symbol map across many languages with low setup cost.


## Investigate the possibility of giving the sandbox access to a local k8s cluster
_Covered by: FR66 (exploratory) → **Epic 15 (Research/POC): Local Kubernetes Cluster Integration** (Story 15.1 spike). Productionization gated on spike outcome — no implementation committed yet._

As we are coding solutions that will be provisioned in k8s (EKS/Openshift) it would be beneficial for the agent to have access to a local k8s cluster to test/validate changes before commiting them. We don't have any ideas yet of what is the best way to achieve this so we will treat this as an exploratory feature/POC using different methodologies to evaluate. Some ideas could be:
- `kind` cluster running directly inside the sandbox. Completely isolated but not accessible to host machine.
- `k3s/kind` cluster provisioned outside the sandbox. Not isolated, kube config injected into the sandbox for access. Less secure than running in the sandbox, but disposable.
- `other` needs research for additional options
