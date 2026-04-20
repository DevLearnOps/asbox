package integration

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestToolchain_devopsValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)

	image := buildTestImage(t)
	container := startTestContainer(ctx, t, image)

	tests := []struct {
		name            string
		command         []string
		expectSubstring string
	}{
		{name: "kubectl", command: []string{"kubectl", "version", "--client=true", "--output=yaml"}, expectSubstring: "gitVersion: v1.35.4"},
		{name: "helm", command: []string{"helm", "version", "--short"}, expectSubstring: "v3.20.2"},
		{name: "kustomize", command: []string{"kustomize", "version"}, expectSubstring: "v5.8.1"},
		{name: "yq", command: []string{"yq", "--version"}, expectSubstring: "4.53.2"},
		{name: "jq", command: []string{"jq", "--version"}, expectSubstring: "jq-"},
		{name: "tofu", command: []string{"tofu", "--version"}, expectSubstring: "OpenTofu v1.11.6"},
		{name: "tflint", command: []string{"tflint", "--version"}, expectSubstring: "0.62.0"},
		{name: "kubeconform", command: []string{"kubeconform", "-v"}, expectSubstring: "v0.7.0"},
		{name: "kube-linter", command: []string{"kube-linter", "version"}, expectSubstring: "0.8.3"},
		{name: "trivy", command: []string{"trivy", "--version"}, expectSubstring: "Version: 0.70.0"},
		{name: "flux", command: []string{"flux", "--version"}, expectSubstring: "flux version 2.8.5"},
		{name: "sops", command: []string{"sops", "--version"}, expectSubstring: "3.12.2"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, exitCode := execAsUser(ctx, t, container, "sandbox", tt.command)
			if exitCode != 0 {
				t.Fatalf("%s exited with %d:\n%s", tt.name, exitCode, truncateOutput(stdout, 500))
			}
			if !strings.Contains(stdout, tt.expectSubstring) {
				t.Errorf("%s output missing %q:\n%s", tt.name, tt.expectSubstring, truncateOutput(stdout, 500))
			}
		})
	}
}

func TestToolchain_codeExploration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)

	image := buildTestImage(t)
	container := startTestContainer(ctx, t, image)

	tests := []struct {
		name            string
		command         []string
		expectSubstring string
	}{
		{name: "ripgrep", command: []string{"rg", "--version"}, expectSubstring: "ripgrep 14.1.0"},
		{name: "fd", command: []string{"fd", "--version"}, expectSubstring: "fdfind 9.0.0"},
		{name: "ast-grep", command: []string{"ast-grep", "--version"}, expectSubstring: "ast-grep 0.42.1"},
		{name: "ctags", command: []string{"ctags", "--version"}, expectSubstring: "Universal Ctags"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout, exitCode := execAsUser(ctx, t, container, "sandbox", tt.command)
			if exitCode != 0 {
				t.Fatalf("%s exited with %d:\n%s", tt.name, exitCode, truncateOutput(stdout, 500))
			}
			if !strings.Contains(stdout, tt.expectSubstring) {
				t.Errorf("%s output missing %q:\n%s", tt.name, tt.expectSubstring, truncateOutput(stdout, 500))
			}
		})
	}
}

func TestToolchain_cacheDirsOwnedBySandbox(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)

	image := buildTestImage(t)
	container := startTestContainer(ctx, t, image)

	statCommand := []string{"bash", "-lc", "stat -c '%U:%G' /home/sandbox/.cache/trivy /home/sandbox/.cache/helm /home/sandbox/.kube /home/sandbox/.terraform.d /home/sandbox/.config/sops"}
	stdout, exitCode := execAsUser(ctx, t, container, "sandbox", statCommand)
	if exitCode != 0 {
		t.Fatalf("stat exited with %d:\n%s", exitCode, truncateOutput(stdout, 500))
	}

	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if line != "sandbox:sandbox" {
			t.Errorf("expected sandbox ownership, got %q", line)
		}
	}

	writableCommand := []string{"bash", "-lc", "set -e; for path in /home/sandbox/.cache/trivy /home/sandbox/.cache/helm /home/sandbox/.kube /home/sandbox/.terraform.d /home/sandbox/.config/sops; do test -w \"$path\"; done"}
	stdout, exitCode = execAsUser(ctx, t, container, "sandbox", writableCommand)
	if exitCode != 0 {
		t.Fatalf("expected validation tool directories to be writable:\n%s", truncateOutput(stdout, 500))
	}

	for _, cmd := range [][]string{
		{"helm", "env"},
		{"kubectl", "config", "view"},
	} {
		stdout, exitCode = execAsUser(ctx, t, container, "sandbox", cmd)
		if exitCode != 0 {
			t.Fatalf("%v exited with %d:\n%s", cmd, exitCode, truncateOutput(stdout, 500))
		}
	}
}

func TestToolchain_rgRespectsGitignore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)

	image := buildTestImage(t)
	container := startTestContainer(ctx, t, image)

	stdout, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"bash", "-lc", `
set -eu
tmp="$(mktemp -d)"
mkdir -p "$tmp/src"
printf 'needle\n' > "$tmp/src/hit.txt"
printf 'needle\n' > "$tmp/ignored.log"
printf '*.log\n' > "$tmp/.gitignore"
(cd "$tmp" && git init -q && rg -n --no-heading needle)
`})
	if exitCode != 0 {
		t.Fatalf("rg exited with %d:\n%s", exitCode, truncateOutput(stdout, 500))
	}
	if !strings.Contains(stdout, "src/hit.txt") {
		t.Errorf("expected rg output to include tracked match:\n%s", truncateOutput(stdout, 500))
	}
	if strings.Contains(stdout, "ignored.log") {
		t.Errorf("expected rg output to respect .gitignore:\n%s", truncateOutput(stdout, 500))
	}
}

func TestToolchain_fdRespectsGitignore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)

	image := buildTestImage(t)
	container := startTestContainer(ctx, t, image)

	stdout, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"bash", "-lc", `
set -eu
tmp="$(mktemp -d)"
mkdir -p "$tmp/src"
: > "$tmp/src/keep.go"
: > "$tmp/ignored.go"
printf 'ignored.go\n' > "$tmp/.gitignore"
(cd "$tmp" && git init -q && fd -e go)
`})
	if exitCode != 0 {
		t.Fatalf("fd exited with %d:\n%s", exitCode, truncateOutput(stdout, 500))
	}
	if !strings.Contains(stdout, "src/keep.go") {
		t.Errorf("expected fd output to include Go file:\n%s", truncateOutput(stdout, 500))
	}
	if strings.Contains(stdout, "ignored.go") {
		t.Errorf("expected fd output to respect .gitignore:\n%s", truncateOutput(stdout, 500))
	}
}

func TestToolchain_helmTemplateOffline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)

	image := buildTestImage(t)
	container := startTestContainer(ctx, t, image)

	t.Run("helm_template", func(t *testing.T) {
		stdout, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"bash", "-lc", `
set -eu
chart_dir="$(mktemp -d)"
mkdir -p "$chart_dir/templates"
cat > "$chart_dir/Chart.yaml" <<'EOF'
apiVersion: v2
name: smoke
version: 0.1.0
EOF
cat > "$chart_dir/values.yaml" <<'EOF'
message: hello
EOF
cat > "$chart_dir/templates/configmap.yaml" <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-config
data:
  message: {{ .Values.message | quote }}
EOF
helm template release-name "$chart_dir"
`})
		if exitCode != 0 {
			t.Fatalf("helm template exited with %d:\n%s", exitCode, truncateOutput(stdout, 500))
		}
		if !strings.Contains(stdout, "apiVersion: v1") {
			t.Errorf("expected rendered chart output, got:\n%s", truncateOutput(stdout, 500))
		}
	})

	t.Run("kustomize_build", func(t *testing.T) {
		stdout, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"bash", "-lc", `
set -eu
kustomize_dir="$(mktemp -d)"
cat > "$kustomize_dir/kustomization.yaml" <<'EOF'
resources:
  - deployment.yaml
EOF
cat > "$kustomize_dir/deployment.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: hello
spec:
  selector:
    matchLabels:
      app: hello
  template:
    metadata:
      labels:
        app: hello
    spec:
      containers:
        - name: hello
          image: nginx:1.27
EOF
kustomize build "$kustomize_dir"
`})
		if exitCode != 0 {
			t.Fatalf("kustomize build exited with %d:\n%s", exitCode, truncateOutput(stdout, 500))
		}
		if !strings.Contains(stdout, "apiVersion: apps/v1") {
			t.Errorf("expected rendered kustomize output, got:\n%s", truncateOutput(stdout, 500))
		}
	})
}

func TestToolchain_astGrepStructuralMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	t.Cleanup(cancel)

	image := buildTestImage(t)
	container := startTestContainer(ctx, t, image)

	stdout, exitCode := execAsUser(ctx, t, container, "sandbox", []string{"bash", "-lc", `
set -eu
tmp="$(mktemp -d)"
cat > "$tmp/demo.js" <<'EOF'
console.log("hello");
console.warn("hi");
EOF
ast-grep run -p 'console.log($X)' --lang js "$tmp"
`})
	if exitCode != 0 {
		t.Fatalf("ast-grep exited with %d:\n%s", exitCode, truncateOutput(stdout, 500))
	}
	if !strings.Contains(stdout, "demo.js") {
		t.Errorf("expected ast-grep output to include file path:\n%s", truncateOutput(stdout, 500))
	}
	if strings.Contains(stdout, "console.warn") {
		t.Errorf("expected ast-grep output to exclude non-matching structural nodes:\n%s", truncateOutput(stdout, 500))
	}
}

func truncateOutput(output string, max int) string {
	if len(output) <= max {
		return output
	}
	return output[:max] + "...(truncated)"
}
