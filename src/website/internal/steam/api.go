package steam

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNoAPIKey means STEAM_WEB_API_KEY isn't configured -- callers treat it
// as "keep whatever's cached", never as a failure.
var ErrNoAPIKey = errors.New("steam: no Web API key configured")

// maxIDsPerCall is the documented per-request limit for both endpoints.
const maxIDsPerCall = 100

const apiBase = "https://api.steampowered.com"

type Client struct {
	Key  string
	HTTP *http.Client
}

func NewClient(key string) *Client {
	return &Client{Key: key, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Summary is the subset of GetPlayerSummaries this app uses. CreatedAt is
// zero when the profile is private (Steam omits timecreated then).
type Summary struct {
	SteamID    string
	Name       string
	AvatarURL  string
	ProfileURL string
	CreatedAt  time.Time
}

// Bans is GetPlayerBans for one account. Works for private profiles too.
type Bans struct {
	SteamID          string
	CommunityBanned  bool
	VACBans          int
	GameBans         int
	DaysSinceLastBan int
}

func (c *Client) get(ctx context.Context, path string, ids []string, out any) error {
	if c == nil || c.Key == "" {
		return ErrNoAPIKey
	}
	q := url.Values{"key": {c.Key}, "steamids": {strings.Join(ids, ",")}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		// Deliberately not wrapping err: *url.Error would include the full
		// request URL, which carries the API key, straight into our logs.
		return fmt.Errorf("steam: request to %s failed", path)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("steam: %s returned HTTP %d", path, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Summaries fetches profiles for up to 100 Steam64 IDs, keyed by ID.
func (c *Client) Summaries(ctx context.Context, ids []string) (map[string]Summary, error) {
	if len(ids) > maxIDsPerCall {
		return nil, fmt.Errorf("steam: %d IDs exceeds the per-call limit of %d", len(ids), maxIDsPerCall)
	}
	var body struct {
		Response struct {
			Players []struct {
				SteamID     string `json:"steamid"`
				PersonaName string `json:"personaname"`
				AvatarFull  string `json:"avatarfull"`
				ProfileURL  string `json:"profileurl"`
				TimeCreated int64  `json:"timecreated"`
			} `json:"players"`
		} `json:"response"`
	}
	if err := c.get(ctx, "/ISteamUser/GetPlayerSummaries/v2/", ids, &body); err != nil {
		return nil, err
	}
	out := make(map[string]Summary, len(body.Response.Players))
	for _, p := range body.Response.Players {
		s := Summary{SteamID: p.SteamID, Name: p.PersonaName, AvatarURL: p.AvatarFull, ProfileURL: p.ProfileURL}
		if p.TimeCreated > 0 {
			s.CreatedAt = time.Unix(p.TimeCreated, 0).UTC()
		}
		out[p.SteamID] = s
	}
	return out, nil
}

// PlayerBans fetches ban status for up to 100 Steam64 IDs, keyed by ID.
func (c *Client) PlayerBans(ctx context.Context, ids []string) (map[string]Bans, error) {
	if len(ids) > maxIDsPerCall {
		return nil, fmt.Errorf("steam: %d IDs exceeds the per-call limit of %d", len(ids), maxIDsPerCall)
	}
	var body struct {
		Players []struct {
			SteamID          string `json:"SteamId"`
			CommunityBanned  bool   `json:"CommunityBanned"`
			NumberOfVACBans  int    `json:"NumberOfVACBans"`
			NumberOfGameBans int    `json:"NumberOfGameBans"`
			DaysSinceLastBan int    `json:"DaysSinceLastBan"`
		} `json:"players"`
	}
	if err := c.get(ctx, "/ISteamUser/GetPlayerBans/v1/", ids, &body); err != nil {
		return nil, err
	}
	out := make(map[string]Bans, len(body.Players))
	for _, p := range body.Players {
		out[p.SteamID] = Bans{
			SteamID: p.SteamID, CommunityBanned: p.CommunityBanned,
			VACBans: p.NumberOfVACBans, GameBans: p.NumberOfGameBans, DaysSinceLastBan: p.DaysSinceLastBan,
		}
	}
	return out, nil
}
