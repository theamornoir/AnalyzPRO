package states

type State string

const (
	StateIdle                 State = "idle"
	StateWaitingAnalysisFile State = "waiting_analysis_file"
)
