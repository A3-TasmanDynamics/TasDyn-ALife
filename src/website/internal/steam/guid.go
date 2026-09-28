// Package steam holds this app's Steam-side integration beyond login
// (login itself is internal/auth/steam.go): BattlEye GUIDs derived from a
// Steam64 ID, and the Steam Web API profile/ban cache. See
// docs/INTEGRATIONS.md §2.1-2.2.
package steam

import (
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
)

// BEGUID returns the BattlEye GUID for a Steam64 ID: the lowercase hex MD5
// of "BE" followed by the ID as a little-endian uint64. Verified against two
// independently published Steam64/GUID pairs in guid_test.go -- BattlEye's
// kick/ban messages only ever show this GUID, so staff need it to be exact.
func BEGUID(steam64 string) (string, error) {
	id, err := strconv.ParseUint(steam64, 10, 64)
	if err != nil {
		return "", fmt.Errorf("steam: invalid Steam64 ID %q: %w", steam64, err)
	}
	buf := make([]byte, 10)
	buf[0], buf[1] = 'B', 'E'
	binary.LittleEndian.PutUint64(buf[2:], id)
	sum := md5.Sum(buf)
	return hex.EncodeToString(sum[:]), nil
}
