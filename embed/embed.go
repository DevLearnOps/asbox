package embed

import "embed"

//go:embed Dockerfile.tmpl entrypoint.sh git-wrapper.sh healthcheck-poller.sh agent-instructions.md.tmpl config.yaml
var Assets embed.FS
