package manifest

type DisabledPersister struct {
}

func (p *DisabledPersister) PersistManifest(deploymentName, manifestName string, data []byte) {}
