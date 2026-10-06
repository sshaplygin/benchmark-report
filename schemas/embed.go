// Package schemas embeds the frozen version 1 document schemas.
package schemas

import (
	"embed"
	"fmt"
)

//go:embed *.schema.json
var files embed.FS

func Read(name string) ([]byte, error) {
	switch name {
	case "report-result", "report-action-inputs", "report-action-outputs", "input-manifest", "normalized-run", "comparison", "presentation", "reproduction", "configuration":
		return files.ReadFile(name + ".schema.json")
	default:
		return nil, fmt.Errorf("unknown schema %q", name)
	}
}
