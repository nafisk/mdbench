package fixture

import "testing"

func TestCatalogHasStableBuiltInSnapshots(t *testing.T) {
	first, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Catalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 4 {
		t.Fatalf("catalog has %d fixtures, want 4", len(first))
	}
	for index := range first {
		if first[index].ID != second[index].ID || first[index].ContentSHA != second[index].ContentSHA {
			t.Fatalf("fixture %d is not stable", index)
		}
	}
	if len(first[0].Files) != 0 || len(first[1].Files) == 0 {
		t.Fatal("empty and populated fixture snapshots are incorrect")
	}
}
