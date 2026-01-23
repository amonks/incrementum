package job

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateJobChangeFallsBackToRoot(t *testing.T) {
	binDir := t.TempDir()
	jjPath := filepath.Join(binDir, "jj")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"new\" ]; then\n" +
		"  if [ \"$2\" = \"trunk()\" ]; then\n" +
		"    echo \"Revision \\`\\\"trunk()\\\"\\` doesn't exist\" >&2\n" +
		"    exit 1\n" +
		"  fi\n" +
		"  if [ \"$2\" = \"root()\" ]; then\n" +
		"    exit 0\n" +
		"  fi\n" +
		"  echo \"unexpected rev\" >&2\n" +
		"  exit 1\n" +
		"fi\n" +
		"if [ \"$1\" = \"log\" ]; then\n" +
		"  echo \"change-id\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"echo \"unexpected args\" >&2\n" +
		"exit 1\n"

	if err := os.WriteFile(jjPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write jj stub: %v", err)
	}

	pathValue := binDir + string(os.PathListSeparator) + os.Getenv("PATH")
	t.Setenv("PATH", pathValue)

	if err := createJobChange(t.TempDir(), "trunk()"); err != nil {
		t.Fatalf("expected fallback to root: %v", err)
	}
}
