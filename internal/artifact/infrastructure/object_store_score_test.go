package infrastructure

import (
	"context"
	"testing"
)

func TestObjectStorePutCopiesInput(t *testing.T) {
	store := NewMemoryObjectStore()
	input := []byte("module")
	if err := store.Put(context.Background(), "m", input); err != nil {
		t.Fatal(err)
	}
	input[0] = 'x'
	got, err := store.Get(context.Background(), "m")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "module" {
		t.Fatalf("stored object changed to %q", got)
	}
}

func TestObjectStoreGetReturnsSnapshot(t *testing.T) {
	store := NewMemoryObjectStore()
	if err := store.Put(context.Background(), "m", []byte("module")); err != nil {
		t.Fatal(err)
	}
	first, err := store.Get(context.Background(), "m")
	if err != nil {
		t.Fatal(err)
	}
	first[0] = 'x'
	second, err := store.Get(context.Background(), "m")
	if err != nil {
		t.Fatal(err)
	}
	if string(second) != "module" {
		t.Fatalf("stored object changed to %q", second)
	}
}
