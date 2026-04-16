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
