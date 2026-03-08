// path: internal/pkg/paths/paths.go (create it)
package paths

import (
	"os"
	"path/filepath"
)

func ModelDir() string {
	if d := os.Getenv("MODEL_DIR"); d != "" {
		return d
	}
	return "/data/models" // default for Fly volume
}

func RuntimeDir() string {
	if d := os.Getenv("ONNX_RUNTIME_PATH"); d != "" {
		return d
	}
	return "/usr/lib" // default for Fly volume
}

func GoogleCredsDir() string {
	if d := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); d != "" {
		return d
	}
	return "/etc/creds" // default for Fly volume
}

func BatchDir(jobID string) string {

	root := filepath.Join(os.TempDir(), "schma-batch")
	_ = os.MkdirAll(root, 0o755)
	dir := filepath.Join(root, jobID)
	_ = os.MkdirAll(dir, 0o755)
	return dir

}
