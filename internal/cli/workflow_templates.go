package cli

import _ "embed"

var (
	//go:embed templates/spec.md
	workflowSpecTemplate string
	//go:embed templates/tasks.md
	workflowTasksTemplate string
	//go:embed templates/HANDOFF.md
	workflowHandoffTemplate string
	//go:embed templates/NIGHT_SHIFT.md
	workflowNightShiftPrompt string
	//go:embed templates/PROACTIVE_OPERATOR.md
	workflowProactivePrompt string
	//go:embed templates/HEARTBEAT.md
	workflowHeartbeatPrompt string
)
