package opencode

import "github.com/amonks/incrementum/internal/todoenv"

func ensureTodoProposerEnv(env []string) []string {
	return replaceEnvVar(env, todoenv.ProposerEnvVar, "true")
}
