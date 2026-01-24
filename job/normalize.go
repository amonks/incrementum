package job

import "strings"

func normalizeStage(stage Stage) Stage {
	return Stage(strings.ToLower(string(stage)))
}

func normalizeStatus(status Status) Status {
	return Status(strings.ToLower(string(status)))
}
