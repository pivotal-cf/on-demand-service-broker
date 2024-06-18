package manifest

import (
	"compress/gzip"
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
	path := filepath.Join(directory, manifestName+".gz")
	if err := os.Mkdir(directory, 0750); err != nil && !os.IsExist(err) {
		p.Logger.Printf("Failed to create directory for persisted manifest %s: %s", path, err)
		return
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		p.Logger.Printf("Failed to persist manifest %s: %s", path, err)
		return
	}
	defer func() {
		if err = f.Close(); err != nil {
			p.Logger.Printf("Failed to close persisted manifest file handle for %s: %s", path, err)
		}
	}()

	compressedWriter := gzip.NewWriter(f)
	if _, err := compressedWriter.Write(data); err != nil {
		p.Logger.Printf("Failed to write compressed data for %s: %s", path, err)
	}

	if err := compressedWriter.Close(); err != nil {
		p.Logger.Printf("Failed to close compressed data stream for %s: %s", path, err)
	}
}

func (p *Persister) Cleanup(deploymentName string) {
	directory := filepath.Join(p.Prefix, deploymentName)

	if err := os.RemoveAll(directory); err != nil {
		p.Logger.Printf("Failed to cleanup persisted manifests directory for %s: %s", directory, err)
	}
}
