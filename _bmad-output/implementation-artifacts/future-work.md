# Future Work

## Shorthand CLI arguments to improve user experience

Simple targeted story to allow users some quick access to running different agents than the default.
- `asbox run -a <agent>`
- agent override should also be available with no argument. We accept the overload in favor of saving the developer the friction of using a flag (`asbox run codex`).


## BMAD Workflow Repository Status Management

When working on changes that affect multiple projects it can be painful to manage git branching and stale changes across all of them. The agent is fully capable of stashing changes or move to a feature branch across all of them autonomously but:
- it needs specific and unequivocal instructions. AGENTS.md files should contain instructions of how to manage branching to resume work.
- sandboxed agent is unable to fetch upstream state, need to explore options to perform this operation on `asbox run` or create a specialised command like `asbox fetch` to ensure local repos have all information from remote even on non-current branches.


## Installation of common devops tools for implementation validation

Common devops tools should be installed by default into the sandbox to allow the agent to self-validate its work. Example of such tools are: git, kubectl, helm, kustomize, yq, jq, opentofu, tflint, kubeconform, kube-linter, trivy, flux, sops.
Tools don't need credentials or authentication information to work in the sandbox. They will be primarily used to validate work like formatting terraform code or test rendering of helm charts.
Installed versions should be pinned to specific current latest versions.
For tools that need to download artefacts like trivy databases we should ensure the sandbox user has appropriate permissions to download them into their default location (typically user home dir dot-folder).


## Installation of agent tools for repository exploration

We want to make sure the sandbox has the necessary CLI tools for code repository exploration and file searches to aid completion of coding tasks.
Installed versions should be pinned to specific current latest versions.
- ripgrep is the standard fast recursive code search tool and respects .gitignore.
- fd is a fast parallel file finder and is much better than find for agent-style traversal.
- git ls-files is often the best starting point for repo tree construction because it avoids a lot of junk automatically.
- ast-grep helps when plain text search is too noisy and the agent needs structural matches.
- universal-ctags gives a broad symbol map across many languages with low setup cost.


## Investigate the possibility of giving the sandbox access to a local k8s cluster

As we are coding solutions that will be provisioned in k8s (EKS/Openshift) it would be beneficial for the agent to have access to a local k8s cluster to test/validate changes before commiting them. We don't have any ideas yet of what is the best way to achieve this so we will treat this as an exploratory feature/POC using different methodologies to evaluate. Some ideas could be:
- `kind` cluster running directly inside the sandbox. Completely isolated but not accessible to host machine.
- `k3s/kind` cluster provisioned outside the sandbox. Not isolated, kube config injected into the sandbox for access. Less secure than running in the sandbox, but disposable.
- `other` needs research for additional options
