//go:build !wasm

package lab

import "webtyp.com/artifacts"

// files are the model files under ModelsDir, by artifact id.
var files = []struct{ id, file string }{
	{DeciderWeights, "decider-0.8b.wtypw"},
	{DeciderMerges, "decider-0.8b.merges"},
	{WriterWeights, "lfm2.5-350m.wtypw"},
	{WriterMerges, "lfm2.5-350m.merges"},
}

// ArtifactSources declares the lab's model files to sitec, which measures them, writes
// /artifacts.json and places them under /artifacts/ in a release build (D-PWA-15). They need
// nothing from the device: the lab must start everywhere (only the space check applies).
func ArtifactSources() []artifacts.Source {
	out := make([]artifacts.Source, len(files))
	for i, f := range files {
		out[i] = artifacts.Source{ID: f.id, Version: ModelsVersion, File: ModelsDir + "/" + f.file}
	}
	return out
}
