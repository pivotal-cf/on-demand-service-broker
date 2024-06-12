package manifest

import (
	"log"
	"os"
	"path/filepath"
)

type Persister struct {
	Prefix string
	Logger *log.Logger
}

func (p *Persister) PersistManifest(deploymentName, manifestName string, data []byte) {
	directory := filepath.Join(p.Prefix, deploymentName)
	if err := os.Mkdir(directory, 0750); err != nil && !os.IsExist(err) {
		p.Logger.Printf("Failed to create directory %s: %s", directory, err)
	}
	path := filepath.Join(directory, manifestName)
	if err := os.WriteFile(path, data, 0640); err != nil {
		p.Logger.Printf("Failed to persist manifest %s: %s", path, err)
	}
}
