package status

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// serverInfo is the subset of a Steam A2S_INFO reply the status page uses.
type serverInfo struct {
	Name       string
	Players    int
	MaxPlayers int
}

var a2sInfoRequest = append([]byte{0xFF, 0xFF, 0xFF, 0xFF, 'T'}, []byte("Source Engine Query\x00")...)

// queryA2SInfo asks a Steam game server (Arma 3's query port -- game port
// + 1 by default) for its A2S_INFO. This is a real "is the game server up
// and answering" signal, unlike a DB query, which says nothing about
// whether arma3server_x64.exe is running.
//
// Handles the challenge handshake Valve added in 2020: the server may first
// answer 'A' (0x41) with a 4-byte challenge, which must be appended to the
// request and resent.
func queryA2SInfo(ctx context.Context, addr string) (serverInfo, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", addr)
	if err != nil {
		return serverInfo{}, err
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(3 * time.Second)
	}
	_ = conn.SetDeadline(deadline)

	buf := make([]byte, 1400)
	req := a2sInfoRequest
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := conn.Write(req); err != nil {
			return serverInfo{}, err
		}
		n, err := conn.Read(buf)
		if err != nil {
			return serverInfo{}, err
		}
		if n < 5 || !bytes.Equal(buf[:4], []byte{0xFF, 0xFF, 0xFF, 0xFF}) {
			return serverInfo{}, errors.New("a2s: unexpected (possibly split) reply")
		}
		switch buf[4] {
		case 0x41: // challenge
			if n < 9 {
				return serverInfo{}, errors.New("a2s: short challenge reply")
			}
			req = append(append([]byte{}, a2sInfoRequest...), buf[5:9]...)
			continue
		case 0x49: // info
			return parseA2SInfo(buf[5:n])
		default:
			return serverInfo{}, fmt.Errorf("a2s: unexpected reply type 0x%02x", buf[4])
		}
	}
	return serverInfo{}, errors.New("a2s: no info reply after challenge")
}

func parseA2SInfo(b []byte) (serverInfo, error) {
	// protocol(1) name map folder game (null-terminated) id(2) players(1) max(1) ...
	if len(b) < 1 {
		return serverInfo{}, errors.New("a2s: empty info")
	}
	p := 1
	readString := func() (string, error) {
		i := bytes.IndexByte(b[p:], 0)
		if i < 0 {
			return "", errors.New("a2s: truncated string")
		}
		s := string(b[p : p+i])
		p += i + 1
		return s, nil
	}
	var info serverInfo
	var err error
	if info.Name, err = readString(); err != nil {
		return info, err
	}
	for i := 0; i < 3; i++ { // map, folder, game
		if _, err = readString(); err != nil {
			return info, err
		}
	}
	if len(b) < p+4 {
		return info, errors.New("a2s: truncated player counts")
	}
	p += 2 // app id
	info.Players = int(b[p])
	info.MaxPlayers = int(b[p+1])
	return info, nil
}
