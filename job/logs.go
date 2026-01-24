package job

import (
	"os"
)

// LogSnapshot returns the stored job event log.
func LogSnapshot(jobID string, opts EventLogOptions) (string, error) {
	path, err := eventLogPath(jobID, opts)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
