package anexos

import "testing"

func TestColumns(t *testing.T) {
	for _, tt := range []struct {
		name  string
		index int
	}{{"A", 1}, {"Z", 26}, {"AA", 27}, {"AB", 28}, {"AZ", 52}, {"BA", 53}} {
		if got := colIndex(tt.name); got != tt.index {
			t.Errorf("colIndex(%q) = %d, quer %d", tt.name, got, tt.index)
		}
		if got := colName(tt.index); got != tt.name {
			t.Errorf("colName(%d) = %q, quer %q", tt.index, got, tt.name)
		}
	}
}

func TestSpread(t *testing.T) {
	s := sheet{cells: map[string]string{"B4": "caminho/", "C4": "campo", "C5": "proprio"}}
	if err := s.spread("B4:C5"); err != nil {
		t.Fatal(err)
	}
	// Only the empty cells take the value of B4; the ones with their own
	// text keep it.
	want := map[string]string{"B4": "caminho/", "B5": "caminho/", "C4": "campo", "C5": "proprio"}
	for ref, v := range want {
		if s.cells[ref] != v {
			t.Errorf("%s = %q, quer %q", ref, s.cells[ref], v)
		}
	}
}
