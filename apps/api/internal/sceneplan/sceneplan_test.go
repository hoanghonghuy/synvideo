package sceneplan_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/hoanghonghuy/synvideo/apps/api/internal/sceneplan"
	"github.com/hoanghonghuy/synvideo/apps/api/internal/script"
)

func TestContentValidatesAgainstApprovedScriptAndPreservesCanonicalNarration(t *testing.T) {
	source := approvedScript()
	content := sceneplan.Content{Scenes: []sceneplan.Scene{
		{Key: "intro-a", ScriptSectionKey: "intro", Narration: "Xin chao", VisualInstruction: "Mo canh", PlannedSourceType: sceneplan.SourceTypeCreatorMedia, ExpectedDurationSeconds: 4},
		{Key: "intro-b", ScriptSectionKey: "intro", Narration: "  the gioi ", VisualInstruction: "Canh rong", PlannedSourceType: sceneplan.SourceTypeStock, ExpectedDurationSeconds: 5},
		{Key: "main", ScriptSectionKey: "main", Narration: "Noi dung chinh", VisualInstruction: "Canh minh hoa", PlannedSourceType: sceneplan.SourceTypeGeneratedImage, ExpectedDurationSeconds: 8},
	}}

	if err := sceneplan.ValidateContentAgainstScript(&content, source); err != nil {
		t.Fatalf("validate content: %v", err)
	}
	if content.Scenes[0].Narration != "Xin chao" || content.Scenes[1].Narration != "  the gioi " {
		t.Fatalf("narration was rewritten: %#v", content.Scenes)
	}
}

func TestContentRejectsNarrationCoverageDrift(t *testing.T) {
	for name, narration := range map[string]string{
		"omitted":     "Xin chao",
		"added":       "Xin chao the gioi them",
		"paraphrased": "Xin chao hanh tinh",
	} {
		t.Run(name, func(t *testing.T) {
			content := validContent()
			content.Scenes[1].Narration = narration
			if err := sceneplan.ValidateContentAgainstScript(&content, approvedScript()); err == nil {
				t.Fatal("expected narration coverage validation error")
			}
		})
	}
}

func TestContentRejectsReorderedOrNonContiguousScriptSections(t *testing.T) {
	content := validContent()
	content.Scenes = append(content.Scenes[:1], sceneplan.Scene{
		Key: "main-first", ScriptSectionKey: "main", Narration: "Noi dung chinh", VisualInstruction: "Canh minh hoa", PlannedSourceType: sceneplan.SourceTypeStock, ExpectedDurationSeconds: 8,
	}, sceneplan.Scene{
		Key: "intro-again", ScriptSectionKey: "intro", Narration: "Xin chao the gioi", VisualInstruction: "Canh ket", PlannedSourceType: sceneplan.SourceTypeStock, ExpectedDurationSeconds: 5,
	})

	if err := sceneplan.ValidateContentAgainstScript(&content, approvedScript()); err == nil {
		t.Fatal("expected section ordering validation error")
	}
}

func TestContentValidationRejectsInvalidSceneShape(t *testing.T) {
	tests := map[string]func(*sceneplan.Content){
		"empty scenes":         func(content *sceneplan.Content) { content.Scenes = nil },
		"duplicate key":        func(content *sceneplan.Content) { content.Scenes[1].Key = content.Scenes[0].Key },
		"invalid key":          func(content *sceneplan.Content) { content.Scenes[0].Key = "Intro Scene" },
		"invalid source type":  func(content *sceneplan.Content) { content.Scenes[0].PlannedSourceType = "ai_magic" },
		"missing visual":       func(content *sceneplan.Content) { content.Scenes[0].VisualInstruction = "  " },
		"duration too large":   func(content *sceneplan.Content) { content.Scenes[0].ExpectedDurationSeconds = 3601 },
		"unicode visual limit": func(content *sceneplan.Content) { content.Scenes[0].VisualInstruction = strings.Repeat("你", 5001) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			content := validContent()
			mutate(&content)
			if err := content.NormalizeAndValidate(); err == nil {
				t.Fatal("expected scene validation error")
			}
		})
	}
}

func TestContentValidationRejectsUnapprovedScript(t *testing.T) {
	content := validContent()
	source := approvedScript()
	source.Status = script.StatusDraft
	if err := sceneplan.ValidateContentAgainstScript(&content, source); !errors.Is(err, sceneplan.ErrScriptNotApproved) {
		t.Fatalf("error = %v, want ErrScriptNotApproved", err)
	}
}

func approvedScript() script.Script {
	return script.Script{
		Status:                script.StatusApproved,
		Version:               3,
		SourceProposalVersion: 2,
		Sections: []script.Section{
			{Key: "intro", Body: "Xin chao the gioi"},
			{Key: "main", Body: "Noi dung chinh"},
		},
	}
}

func validContent() sceneplan.Content {
	return sceneplan.Content{Scenes: []sceneplan.Scene{
		{Key: "intro", ScriptSectionKey: "intro", Narration: "Xin chao the gioi", VisualInstruction: "Mo canh", PlannedSourceType: sceneplan.SourceTypeStock, ExpectedDurationSeconds: 5},
		{Key: "main", ScriptSectionKey: "main", Narration: "Noi dung chinh", VisualInstruction: "Canh minh hoa", PlannedSourceType: sceneplan.SourceTypeStock, ExpectedDurationSeconds: 8},
	}}
}
