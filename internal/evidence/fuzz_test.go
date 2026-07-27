package evidence

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/oklog/ulid/v2"
)

func FuzzV1ManifestHashing(f *testing.F) {
	f.Add([]byte("alpha"), []byte("beta"))
	f.Add([]byte{}, []byte{0, 1, 2})
	f.Fuzz(func(t *testing.T, firstContent, secondContent []byte) {
		if len(firstContent)+len(secondContent) > 4096 {
			t.Skip()
		}
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "a"), firstContent, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "b"), secondContent, 0o600); err != nil {
			t.Fatal(err)
		}
		first, err := hashArtifacts(root)
		if err != nil {
			t.Fatal(err)
		}
		second, err := hashArtifacts(root)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("manifest hashing is nondeterministic:\n%+v\n%+v", first, second)
		}
	})
}

func FuzzV1RunIDOrdering(f *testing.F) {
	f.Add(uint8(2))
	f.Add(uint8(16))
	f.Fuzz(func(t *testing.T, requested uint8) {
		count := int(requested%32) + 2
		previous := NewRunID()
		if _, err := ulid.ParseStrict(previous); err != nil {
			t.Fatal(err)
		}
		for index := 1; index < count; index++ {
			current := NewRunID()
			if _, err := ulid.ParseStrict(current); err != nil {
				t.Fatal(err)
			}
			if current <= previous {
				t.Fatalf("run IDs are not strictly ordered: %q then %q", previous, current)
			}
			previous = current
		}
	})
}
