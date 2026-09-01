package sceneplangeneration

type SourceType string

const (
	SourceTypeStock          SourceType = "stock"
	SourceTypeUpload         SourceType = "upload"
	SourceTypeCreatorMedia   SourceType = "creator_media"
	SourceTypeGeneratedImage SourceType = "generated_image"
	SourceTypeGeneratedVideo SourceType = "generated_video"
)

type Scene struct {
	Key                     string
	ScriptSectionKey        string
	Narration               string
	VisualInstruction       string
	PlannedSourceType       SourceType
	ExpectedDurationSeconds int
	CaptionIntent           string
	TransitionNotes         string
}

type Candidate struct {
	SourceScriptVersion   int
	SourceProposalVersion int
	Scenes                []Scene
}
