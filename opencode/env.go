package opencode

const opencodeTodoProposerEnvVar = "INCREMENTUM_TODO_PROPOSER"

func ensureTodoProposerEnv(env []string) []string {
	return replaceEnvVar(env, opencodeTodoProposerEnvVar, "true")
}
