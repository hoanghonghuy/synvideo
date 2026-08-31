package proposalgeneration

// StructureItem is one ordered section in a Proposal candidate.
type StructureItem struct {
	Key     string
	Title   string
	Purpose string
}

// Candidate is validated editable Proposal content plus source brief revision.
type Candidate struct {
	SourceBriefRevision      int
	TitleOptions             []string
	HookOptions              []string
	AudienceSummary          string
	ObjectiveSummary         string
	NarrativeAngle           string
	EstimatedDurationSeconds *int
	FormatRationale          string
	Structure                []StructureItem
	VisualDirection          string
	VoiceDirection           string
	MusicDirection           string
	CaptionDirection         string
	CallToAction             string
	ResearchGaps             []string
	Warnings                 []string
}
