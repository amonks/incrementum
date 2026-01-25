package main

import "fmt"

var buildChangeID = "unknown"
var buildCommitID = "unknown"

func init() {
	rootCmd.Version = versionString()
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

func versionString() string {
	return fmt.Sprintf("change_id %s\ncommit_id %s", buildChangeID, buildCommitID)
}
