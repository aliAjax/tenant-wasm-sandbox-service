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
	for _, imported := range candidate.ImportList {
		if strings.TrimSpace(imported) == "" {
			return fmt.Errorf("import name is empty")
		}
		if !v.AllowedImports[imported] {
			return fmt.Errorf("import %q is not allowed", imported)
		}
	}
	seen := map[string]bool{}
	for _, capability := range candidate.Capabilities {
		if seen[capability.Name] {
			return fmt.Errorf("capability %q is duplicated", capability.Name)
		}
		seen[capability.Name] = true
		if !v.AllowedWASI[capability.Name] {
			return fmt.Errorf("WASI capability %q is not allowed", capability.Name)
		}
		switch capability.Access {
		case "read", "write", "read-write":
		default:
			return fmt.Errorf("capability access %q is invalid", capability.Access)
		}
	}
	return nil
}
