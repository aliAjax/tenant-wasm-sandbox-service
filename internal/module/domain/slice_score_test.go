package domain

import (
	"reflect"
	"testing"
	"time"
)

func TestModuleCapabilitiesAreImmutableSnapshot(t *testing.T) {
	caps := make([]Capability, 1, 3)
	caps[0] = Capability{Name: "wasi.clock_time_get", Access: "read"}
	m, err := New("mod-cap-snapshot", RegisterRequest{TenantID: "tenant-a", Name: "clock", Version: "1", Content: []byte("module"), Capabilities: caps}, 1024, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	published, err := m.Publish(time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	caps[0] = Capability{Name: "wasi.random_get", Access: "write"}
	caps = append(caps, Capability{Name: "wasi.poll_oneoff", Access: "read"})
	if want := []Capability{{Name: "wasi.clock_time_get", Access: "read"}}; !reflect.DeepEqual(published.Capabilities, want) {
		t.Fatalf("published capabilities changed: got %#v want %#v", published.Capabilities, want)
	}
}

func TestModuleImportsSurviveCallerReuse(t *testing.T) {
	imports := make([]string, 2, 4)
	copy(imports, []string{"wasi.random_get", "wasi.clock_time_get"})
	m, err := New("mod-import-snapshot", RegisterRequest{TenantID: "tenant-a", Name: "imports", Version: "1", Content: []byte("module"), ImportList: imports}, 1024, time.Unix(200, 0))
	if err != nil {
		t.Fatal(err)
	}
	published, err := m.Publish(time.Unix(201, 0))
	if err != nil {
		t.Fatal(err)
	}
	imports[0] = "host.internal"
	imports = append(imports[:1], "host.socket", "host.secret")
	if want := []string{"wasi.random_get", "wasi.clock_time_get"}; !reflect.DeepEqual(published.ImportList, want) {
		t.Fatalf("published imports changed: got %#v want %#v", published.ImportList, want)
	}
}
