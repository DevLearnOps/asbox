package embed

import (
	"strings"
	"testing"
)

func TestEntrypoint_AgentCmdExecNoShell(t *testing.T) {
	data, err := Assets.ReadFile("entrypoint.sh")
	if err != nil {
		t.Fatalf("reading embedded entrypoint.sh: %v", err)
	}

	script := string(data)
	if strings.Contains(script, `bash -c "${AGENT_CMD}"`) {
		t.Fatal(`entrypoint.sh still contains bash -c "${AGENT_CMD}"`)
	}
	if strings.Contains(script, `bash -c "$AGENT_CMD"`) {
		t.Fatal(`entrypoint.sh still contains bash -c "$AGENT_CMD"`)
	}
	if !strings.Contains(script, `exec gosu sandbox ${AGENT_CMD}`) {
		t.Fatal(`entrypoint.sh does not contain hardened direct exec for ${AGENT_CMD}`)
	}
}
