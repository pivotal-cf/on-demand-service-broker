package manifest

import (
	"compress/gzip"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/pivotal-cf/on-demand-services-sdk/bosh"
	"gopkg.in/yaml.v2"
)

type Persister struct {
	Prefix string
	Logger *log.Logger
}

func (p Persister) PersistManifest(deploymentName, manifestName string, data []byte) {
	data, err := p.SortManifest(data)
	if err != nil {
		p.Logger.Printf("Failed to sort persisted manifest")
		return
	}
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

func (p Persister) Cleanup(deploymentName string) {
	directory := filepath.Join(p.Prefix, deploymentName)

	if err := os.RemoveAll(directory); err != nil {
		p.Logger.Printf("Failed to cleanup persisted manifests directory for %s: %s", directory, err)
	}
}

func (p Persister) SortManifest(contents []byte) ([]byte, error) {
	var m bosh.BoshManifest
	err := yaml.Unmarshal(contents, &m)
	if err != nil {
		return nil, err
	}
	sort.Slice(m.Variables, func(i, j int) bool {
		return m.Variables[i].Name < m.Variables[j].Name
	})
	sort.Slice(m.InstanceGroups, func(i, j int) bool {
		return m.InstanceGroups[i].Name < m.InstanceGroups[j].Name
	})
	sort.Slice(m.Releases, func(i, j int) bool {
		return m.Releases[i].Name < m.Releases[j].Name
	})
	sort.Slice(m.Stemcells, func(i, j int) bool {
		return m.Stemcells[i].Alias < m.Stemcells[j].Alias
	})
	for index := range m.InstanceGroups {
		ig := &m.InstanceGroups[index]

		sort.Slice(ig.Jobs, func(i, j int) bool {
			return ig.Jobs[i].Name < ig.Jobs[j].Name
		})
	}
	return yaml.Marshal(m)
}
