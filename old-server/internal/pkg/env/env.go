package env

import "os"

func ModelDir() string {
	if d := os.Getenv("MODEL_DIR"); d != "" {
		return d
	}
	return "/data/models"
}
