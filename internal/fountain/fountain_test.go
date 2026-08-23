package fountain

import (
	"strings"
	"testing"

	"github.com/bg-dao/axon-codex-dramaops/internal/domain"
)

func TestFountainParseAndSemanticRoundTripPreservesStableIDs(t *testing.T) {
	source := `Title: The Call

.INT. APARTMENT - NIGHT

Lin checks the locked door.

@LIN
(afraid)
Who is there?

[[SFX: Three knocks]]

.EXT. ROOFTOP - DAWN

@LIN (V.O.)
I already knew the answer.
`
	episode, scenes, err := Parse("episode-001", "The Call", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(scenes) != 2 || len(episode.ScriptBlocks) != 4 || episode.ScriptBlocks[1].Kind != domain.ScriptDialogue || episode.ScriptBlocks[1].Emotion != "afraid" || episode.ScriptBlocks[3].Kind != domain.ScriptVoiceOver {
		t.Fatalf("unexpected parse: %+v %+v", episode, scenes)
	}
	for _, value := range append([]string{scenes[0].ID, scenes[1].ID}, episode.ScriptBlocks[0].ID, episode.ScriptBlocks[1].ID, episode.ScriptBlocks[2].ID, episode.ScriptBlocks[3].ID) {
		if value == "" {
			t.Fatal("stable ID was not generated")
		}
	}
	characters := []domain.Character{{ID: "lin-series-id", Name: "Lin"}}
	for index := range episode.ScriptBlocks {
		if episode.ScriptBlocks[index].Kind == domain.ScriptDialogue || episode.ScriptBlocks[index].Kind == domain.ScriptVoiceOver {
			episode.ScriptBlocks[index].CharacterID = "lin-series-id"
		}
	}
	formatted := Format(episode, scenes, characters)
	if !strings.Contains(formatted, "/* dramaops:characterId=lin-series-id */") || !strings.Contains(formatted, "@Lin") {
		t.Fatalf("formatted Fountain omitted semantic metadata:\n%s", formatted)
	}
	roundTrip, roundTripScenes, err := Parse("episode-001", "The Call", formatted)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundTrip.ScriptBlocks) != len(episode.ScriptBlocks) || len(roundTripScenes) != len(scenes) {
		t.Fatalf("round trip shape changed: %+v %+v", roundTrip, roundTripScenes)
	}
	for index := range episode.ScriptBlocks {
		left, right := episode.ScriptBlocks[index], roundTrip.ScriptBlocks[index]
		if left.ID != right.ID || left.SceneID != right.SceneID || left.Kind != right.Kind || left.Text != right.Text || left.CharacterID != right.CharacterID {
			t.Fatalf("block %d changed: %+v -> %+v", index, left, right)
		}
	}
}

func TestFountainRejectsContentBeforeSceneAndGeneratesUniqueDuplicateIDs(t *testing.T) {
	if _, _, err := Parse("episode-001", "Bad", "Action before a scene."); err == nil {
		t.Fatal("content before a scene must fail")
	}
	episode, _, err := Parse("episode-001", "Duplicates", ".INT. ROOM - DAY\n\nSame action.\n\nSame action.\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(episode.ScriptBlocks) != 2 || episode.ScriptBlocks[0].ID == episode.ScriptBlocks[1].ID {
		t.Fatalf("duplicate IDs were not disambiguated: %+v", episode.ScriptBlocks)
	}
}
