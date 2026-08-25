package adapter

import (
	"fmt"
	"strings"

	module "github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

type CapabilityValidator struct {
	AllowedImports map[string]bool
	AllowedWASI    map[string]bool
}

func (v CapabilityValidator) Validate(candidate module.Module) error {
	seen := map[string]bool{}
	for _, raw := range candidate.Capabilities {
		name := strings.TrimSpace(raw.Name)
		access := strings.TrimSpace(raw.Access)
		if name == "" {
			return fmt.Errorf("capability name is empty")
		}
		if seen[name] {
			return fmt.Errorf("capability %q is duplicated", name)
		}
		seen[name] = true
		if !v.AllowedWASI[name] {
			return fmt.Errorf("WASI capability %q is not allowed", name)
		}
		switch access {
		case "read", "write", "read-write":
		default:
			return fmt.Errorf("capability access %q is invalid", access)
		}
	}
	for _, raw := range candidate.ImportList {
		imported := strings.TrimSpace(raw)
		if imported == "" {
			return fmt.Errorf("import name is empty")
		}
		if !v.AllowedImports[imported] {
			return fmt.Errorf("import %q is not allowed", imported)
		}
	}
	return nil
}
