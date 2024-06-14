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
	if err := os.Mkdir(directory, 0750); err != nil && !os.IsExist(err) {
		p.Logger.Printf("Failed to create directory %s: %s", directory, err)
		return
	}
	path := filepath.Join(directory, manifestName+".gz")

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		p.Logger.Printf("Failed to persist manifest %s: %s", path, err)
		return
	}
	defer func() {
		if err = f.Close(); err != nil {
			panic("test me: failed to close manifest file")
		}
	}()

	compressedWriter := gzip.NewWriter(f)
	if _, err := compressedWriter.Write(data); err != nil {
		panic("test me: failed to write compressed data")
	}
	if err := compressedWriter.Close(); err != nil {
		panic("test me: failed to close compressed data stream")
	}
}
