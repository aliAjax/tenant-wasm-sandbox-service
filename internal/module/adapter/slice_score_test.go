package adapter

import (
	"reflect"
	"testing"
	"time"

	module "github.com/acme/wasm-sandbox-executor/internal/module/domain"
)

func TestValidationCannotMutatePublishedModule(t *testing.T) {
	candidate, err := module.New("mod-validator-cap", module.RegisterRequest{TenantID: "tenant-a", Name: "validator-cap", Version: "1", Content: []byte("module"), Capabilities: []module.Capability{{Name: " wasi.random_get ", Access: " read "}, {Name: "wasi.clock_time_get", Access: "read"}}, ImportList: []string{"wasi.random_get"}}, 1024, time.Unix(300, 0))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err = candidate.Publish(time.Unix(301, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := append([]module.Capability(nil), candidate.Capabilities...)
	v := CapabilityValidator{AllowedImports: map[string]bool{"wasi.random_get": true}, AllowedWASI: map[string]bool{"wasi.random_get": true, "wasi.clock_time_get": true}}
	if err := v.Validate(candidate); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(candidate.Capabilities, want) {
		t.Fatalf("validation reordered capabilities: got %#v want %#v", candidate.Capabilities, want)
	}
}

func TestListSnapshotsDoNotShareBackingArrays(t *testing.T) {
	candidate, err := module.New("mod-validator-import", module.RegisterRequest{TenantID: "tenant-a", Name: "validator-import", Version: "1", Content: []byte("module"), ImportList: []string{" wasi.random_get ", "wasi.clock_time_get"}, Capabilities: []module.Capability{{Name: "wasi.random_get", Access: "read"}}}, 1024, time.Unix(400, 0))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err = candidate.Publish(time.Unix(401, 0))
	if err != nil {
		t.Fatal(err)
	}
	want := append([]string(nil), candidate.ImportList...)
	v := CapabilityValidator{AllowedImports: map[string]bool{"wasi.random_get": true, "wasi.clock_time_get": true}, AllowedWASI: map[string]bool{"wasi.random_get": true}}
	if err := v.Validate(candidate); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(candidate.ImportList, want) {
		t.Fatalf("validation reordered imports: got %#v want %#v", candidate.ImportList, want)
	}
}
