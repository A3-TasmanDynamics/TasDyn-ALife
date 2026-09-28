package status

import "testing"

func TestParseA2SInfo(t *testing.T) {
	// protocol, name, map, folder, game, app id (LE), players, max, ...
	b := []byte{0x11}
	b = append(b, "TasDyn-ALife | Altis\x00Altis\x00Arma3\x00Arma 3\x00"...)
	b = append(b, 0x00, 0x00, 7, 64, 0, 'd', 'w', 0, 0)

	info, err := parseA2SInfo(b)
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "TasDyn-ALife | Altis" || info.Players != 7 || info.MaxPlayers != 64 {
		t.Fatalf("got %+v", info)
	}
}

func TestParseA2SInfoTruncated(t *testing.T) {
	if _, err := parseA2SInfo([]byte{0x11, 'x', 0}); err == nil {
		t.Fatal("expected an error for a truncated reply")
	}
}
