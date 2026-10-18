package main

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// TeamSpeak 3 server. The manager doesn't download it: the TS3 server zip
// comes from teamspeak.com, with a licence you accept yourself (the tick
// box on the Services tab). Point TS3Path at the unzipped folder.
//
// Every start sets the serveradmin ServerQuery password to the one in
// services.json (generated the first time), so the manager can always log
// in -- for the online count, and to make a new admin privilege key.
// ServerQuery only listens to 127.0.0.1 by default (query_ip_allowlist.txt),
// which is all the manager needs.

func ts3Start() (launchSpec, error) {
	cfg, err := loadServicesConfig()
	if err != nil {
		return launchSpec{}, err
	}
	if cfg.TS3Path == "" {
		return launchSpec{}, fmt.Errorf("set the TeamSpeak 3 server folder first (the unzipped download from teamspeak.com)")
	}
	exe := filepath.Join(cfg.TS3Path, "ts3server.exe")
	if _, err := os.Stat(exe); err != nil {
		return launchSpec{}, fmt.Errorf("ts3server.exe not found in %s", cfg.TS3Path)
	}
	if !cfg.TS3LicenseAccepted {
		return launchSpec{}, fmt.Errorf("accept the TeamSpeak server licence first (TeamSpeak card, Setup)")
	}
	if cfg.TS3QueryPassword == "" {
		cfg.TS3QueryPassword = randomPassword(20)
		if err := saveServicesConfig(cfg); err != nil {
			return launchSpec{}, err
		}
	}
	cmd := exec.Command(exe,
		"license_accepted=1",
		"serveradmin_password="+cfg.TS3QueryPassword,
		fmt.Sprintf("query_port=%d", cfg.TS3QueryPort),
		fmt.Sprintf("default_voice_port=%d", cfg.TS3VoicePort),
	)
	cmd.Dir = cfg.TS3Path
	return launchSpec{Cmd: cmd, LogToFile: true, Hidden: true}, nil
}

var ts3TokenPattern = regexp.MustCompile(`token=([A-Za-z0-9+/=]{20,})`)

// ts3WatchForKey: on a brand-new TS3 server's first start, it prints a
// one-time "ServerAdmin" privilege key. Saved so the Services tab can show
// it until it's been used.
func ts3WatchForKey(logPath string) {
	for i := 0; i < 60; i++ {
		time.Sleep(time.Second)
		for _, line := range tailFile(logPath, 200) {
			if m := ts3TokenPattern.FindStringSubmatch(line); m != nil {
				_ = updateServicesConfig(func(c *ServicesConfig) { c.TS3PrivilegeKey = m[1] })
				emit("services:changed")
				return
			}
		}
	}
}

func randomPassword(n int) string {
	const chars = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	for i := range b {
		v, _ := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
		b[i] = chars[v.Int64()]
	}
	return string(b)
}

// ---- a minimal ServerQuery client (raw, port 10011, loopback only) ----

// ts3Query logs in as serveradmin, selects virtual server 1 and runs cmds,
// returning each command's first reply line as key/value pairs.
func ts3Query(cmds ...string) ([]map[string]string, error) {
	cfg, err := loadServicesConfig()
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.TS3QueryPort), 3*time.Second)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	r := bufio.NewReader(conn)
	// Banner: "TS3" then a welcome line.
	for i := 0; i < 2; i++ {
		if _, err := r.ReadString('\n'); err != nil {
			return nil, err
		}
	}
	run := func(cmd string) (map[string]string, error) {
		if _, err := fmt.Fprintf(conn, "%s\n", cmd); err != nil {
			return nil, err
		}
		var data map[string]string
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return nil, err
			}
			line = strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(line, "error ") {
				kv := ts3Parse(line)
				if kv["id"] != "0" {
					return nil, fmt.Errorf("ServerQuery: %s", kv["msg"])
				}
				return data, nil
			}
			if data == nil && line != "" {
				data = ts3Parse(strings.SplitN(line, "|", 2)[0])
			}
		}
	}
	if _, err := run("login serveradmin " + ts3Escape(cfg.TS3QueryPassword)); err != nil {
		return nil, err
	}
	if _, err := run("use sid=1"); err != nil {
		return nil, err
	}
	var out []map[string]string
	for _, c := range cmds {
		res, err := run(c)
		if err != nil {
			return out, err
		}
		out = append(out, res)
	}
	_, _ = run("quit")
	return out, nil
}

func ts3Parse(line string) map[string]string {
	kv := map[string]string{}
	for _, part := range strings.Fields(line) {
		k, v, _ := strings.Cut(part, "=")
		kv[k] = ts3Unescape(v)
	}
	return kv
}

var ts3Esc = strings.NewReplacer(`\`, `\\`, `/`, `\/`, " ", `\s`, "|", `\p`, "\n", `\n`, "\r", `\r`, "\t", `\t`)
var ts3Unesc = strings.NewReplacer(`\\`, `\`, `\/`, `/`, `\s`, " ", `\p`, "|", `\n`, "\n", `\r`, "\r", `\t`, "\t")

func ts3Escape(s string) string   { return ts3Esc.Replace(s) }
func ts3Unescape(s string) string { return ts3Unesc.Replace(s) }

// ts3Health: name and online count, via ServerQuery.
func ts3Health() string {
	res, err := ts3Query("serverinfo")
	if err != nil || len(res) == 0 || res[0] == nil {
		if err != nil && strings.Contains(err.Error(), "ServerQuery") {
			return err.Error()
		}
		return "ServerQuery not answering yet"
	}
	info := res[0]
	online := info["virtualserver_clientsonline"]
	if q := info["virtualserver_queryclientsonline"]; q != "" && online != "" {
		var o, qn int
		_, _ = fmt.Sscan(online, &o)
		_, _ = fmt.Sscan(q, &qn)
		online = fmt.Sprint(o - qn) // don't count query connections (like this one)
	}
	return fmt.Sprintf("%s · %s / %s online · voice port %s", info["virtualserver_name"], online, info["virtualserver_maxclients"], info["virtualserver_port"])
}

// ts3NewAdminKey makes a fresh privilege key for the Server Admin group.
func ts3NewAdminKey() (string, error) {
	// "Server Admin" is group 6 on a default TS3 server.
	out, err := ts3Query("tokenadd tokentype=0 tokenid1=6 tokenid2=0 tokendescription=" + ts3Escape("Server Manager admin key"))
	if err != nil {
		return "", err
	}
	if len(out) == 0 || out[0]["token"] == "" {
		return "", fmt.Errorf("TeamSpeak didn't return a key")
	}
	key := out[0]["token"]
	_ = updateServicesConfig(func(c *ServicesConfig) { c.TS3PrivilegeKey = key })
	return key, nil
}
