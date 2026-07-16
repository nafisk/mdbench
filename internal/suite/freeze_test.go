package suite

import (
	"testing"
	"time"
)

func TestFrozenRevisionDoesNotShareDraftState(t *testing.T) {
	draft := validDraft(t)
	frozen, err := Freeze(draft, 1, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	draft.Cases[0].Prompt = "mutated"
	if frozen.Cases[0].Prompt == draft.Cases[0].Prompt {
		t.Fatal("frozen revision shares mutable case state")
	}
	second, err := Freeze(frozen.EditableDraft(), 2, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	if second.ContentSHA == frozen.ContentSHA {
		t.Fatal("different revisions have the same revision hash")
	}
}
