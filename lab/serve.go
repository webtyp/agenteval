//go:build !wasm

package lab

import (
	"fmt"
	"os"
	"path/filepath"

	"webtyp.com/server/httpd"
	"webtyp.com/sitec"
)

const errMissingModel = "agenteval: missing %s — copy or hard-link the model files into %s/ (README, \"Laboratorio\")"

// Serve builds the release of the lab into root/OutDir and serves it over HTTPS on port. It
// blocks. The release is the only build with the Worker binaries and the model files.
func Serve(root, port string) error {
	for _, f := range files {
		p := filepath.Join(root, ModelsDir, f.file)
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf(errMissingModel, p, ModelsDir)
		}
	}
	if err := sitec.Build(root, OutDir); err != nil {
		return err
	}
	return httpd.New(httpd.Config{Port: port, PublicDir: filepath.Join(root, OutDir)}).ListenAndServe()
}
