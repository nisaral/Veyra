package planner

import (
	"strings"
	"testing"

	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
)

func TestCompactIsProposedOnlyAfterTheHistoryGrows(t *testing.T) {
	short := Candidates(Input{State: &veyrav1.CommonExecutionState{
		Messages: []*veyrav1.Message{{Role: "user", Content: "hi"}},
	}, CurrentHarness: "native"})
	for _, c := range short {
		if c.Id == "compact_context" {
			t.Fatal("short history must not offer compaction")
		}
	}

	long := &veyrav1.CommonExecutionState{}
	long.Messages = []*veyrav1.Message{{Role: "user", Content: strings.Repeat("x", compactMinChars)}}
	got := Candidates(Input{State: long, CurrentHarness: "native"})
	found := false
	for _, c := range got {
		if c.Id == "compact_context" {
			found = true
			if c.Type != veyrav1.ActionType_TOOL_CALL {
				t.Fatalf("compact type = %s", c.Type)
			}
		}
	}
	if !found {
		t.Fatal("long history must offer compact_context")
	}
}
