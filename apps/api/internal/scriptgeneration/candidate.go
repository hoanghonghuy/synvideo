package scriptgeneration

// Section represents one ordered script section with a key, optional heading, and body.
type Section struct {
	Key     string
	Heading string
	Body    string
}

// Candidate is validated editable Script content plus the source proposal version.
type Candidate struct {
	SourceProposalVersion    int
	Sections                 []Section
	EstimatedDurationSeconds *int
	Notes                    string
}
