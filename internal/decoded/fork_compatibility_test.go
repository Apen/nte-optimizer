package decoded

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProductionCompatibilityMatchesDataminedArcGroups(t *testing.T) {
	root := filepath.Join("..", "..", "data", "game")
	catalog, err := LoadArcCompatibilityCatalog(
		filepath.Join(root, "characters", "decode.json"),
		filepath.Join(root, "equipment", "arcs.json"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := catalog.CompatibleCharacterIDs("fork_DemonBlade"), []int{1003, 1008, 1036, 1073}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DemonBlade compatible characters = %v, want %v", got, want)
	}
}

func TestLoadArcCompatibilityCatalogMatchesCharacterAndArcGroups(t *testing.T) {
	dir := t.TempDir()
	characters := filepath.Join(dir, "characters.json")
	arcs := filepath.Join(dir, "arcs.json")
	if err := os.WriteFile(characters, []byte(`{"characters":{"1036":{"character_group_type":"CHARACTER_GROUP_TYPE_FOUR"},"1003":{"character_group_type":"CHARACTER_GROUP_TYPE_FOUR"},"1004":{"character_group_type":"CHARACTER_GROUP_TYPE_TWO"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(arcs, []byte(`{"forks":{"fork_DemonBlade":{"apply_group_type":"CHARACTER_GROUP_TYPE_FOUR"},"fork_Rose":{"apply_group_type":"CHARACTER_GROUP_TYPE_TWO"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	catalog, err := LoadArcCompatibilityCatalog(characters, arcs)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := catalog.CompatibleCharacterIDs("fork_DemonBlade"), []int{1003, 1036}; !reflect.DeepEqual(got, want) {
		t.Fatalf("DemonBlade compatible characters = %v, want %v", got, want)
	}
	if got, want := catalog.CompatibleCharacterIDs("fork_Rose"), []int{1004}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Rose compatible characters = %v, want %v", got, want)
	}
	if got := catalog.CompatibleCharacterIDs("unknown"); len(got) != 0 {
		t.Fatalf("unknown Arc compatibility = %v, want empty", got)
	}
}
