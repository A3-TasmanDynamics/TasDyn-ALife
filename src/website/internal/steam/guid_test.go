package steam

import "testing"

// Pairs from two independent sources: the Go package github.com/woozymasta/dzid
// (its documented example) and a community Arma 3 GUID converter.
func TestBEGUIDKnownPairs(t *testing.T) {
	cases := map[string]string{
		"76561198037610867": "430ca61303cc09a0f3da9a490b2233ce",
		"76561197960265728": "4fc867abf98b934e9e7eeaf15170258c",
	}
	for id, want := range cases {
		got, err := BEGUID(id)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if got != want {
			t.Errorf("BEGUID(%s) = %s, want %s", id, got, want)
		}
	}
}

func TestBEGUIDRejectsNonNumeric(t *testing.T) {
	if _, err := BEGUID("not-a-steam-id"); err == nil {
		t.Fatal("expected an error")
	}
}
