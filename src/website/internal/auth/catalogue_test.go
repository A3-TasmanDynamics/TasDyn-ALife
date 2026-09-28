package auth

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

// The schema seeds rank_permissions from a VALUES list that must match the
// catalogue exactly -- otherwise a fresh install would start with different
// permissions than the code expects.
func TestSchemaSeedMatchesCatalogue(t *testing.T) {
	raw, err := os.ReadFile("../../../../database/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	block := regexp.MustCompile(`(?s)INSERT INTO rank_permissions.*?\) AS k\(command_key, seed_level\)`).Find(raw)
	if block == nil {
		t.Fatal("rank_permissions seed not found in schema.sql")
	}
	seeded := map[string]int{}
	for _, m := range regexp.MustCompile(`\('([a-z_.]+)', (\d+)\)`).FindAllSubmatch(block, -1) {
		lvl, _ := strconv.Atoi(string(m[2]))
		seeded[string(m[1])] = lvl
	}
	for _, p := range Catalogue {
		lvl, ok := seeded[p.Key]
		if !ok {
			t.Errorf("%s is in the catalogue but not seeded in schema.sql", p.Key)
		} else if lvl != p.SeedLevel {
			t.Errorf("%s: schema seeds level %d, catalogue says %d", p.Key, lvl, p.SeedLevel)
		}
		delete(seeded, p.Key)
	}
	for k := range seeded {
		t.Errorf("%s is seeded in schema.sql but not in the catalogue", k)
	}
}

func TestCatalogueKeysUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range Catalogue {
		if seen[p.Key] {
			t.Errorf("duplicate key %s", p.Key)
		}
		seen[p.Key] = true
	}
}
