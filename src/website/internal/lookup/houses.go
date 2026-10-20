package lookup

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/gamenames"
)

type HouseRow struct {
	ID      int64
	Key     string
	Name    string
	Price   int64
	Owner   string
	OwnerID int64 // 0 = for sale
	Locked  bool
}

// SearchHouses finds houses by key, owner (name or Steam64) or id; owned
// narrows it ("owned", "forsale", "" = all). Up to 100, owned first.
func SearchHouses(ctx context.Context, pool *pgxpool.Pool, q, owned string) ([]HouseRow, error) {
	q = strings.TrimSpace(q)
	like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q) + "%"
	id, _ := strconv.ParseInt(strings.TrimPrefix(q, "#"), 10, 64)
	rows, err := pool.Query(ctx, `
		SELECT h.id, h.house_key, h.price, COALESCE(`+ownerExpr+`, ''), COALESCE(h.owner_player_id, 0), h.locked
		FROM houses h LEFT JOIN players p ON p.id = h.owner_player_id
		WHERE ($1 = '' OR h.house_key ILIKE $2 OR p.name ILIKE $2 OR p.uid = $1 OR h.id = $3)
		  AND ($4 = '' OR ($4 = 'owned') = (h.owner_player_id IS NOT NULL))
		ORDER BY (h.id = $3) DESC, (h.owner_player_id IS NULL), h.house_key
		LIMIT 100`, q, like, id, owned)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HouseRow
	for rows.Next() {
		var h HouseRow
		if err := rows.Scan(&h.ID, &h.Key, &h.Price, &h.Owner, &h.OwnerID, &h.Locked); err != nil {
			return nil, err
		}
		h.Name = gamenames.House(h.Key)
		out = append(out, h)
	}
	return out, rows.Err()
}

type StoredItem struct {
	Name  string
	Value string
}

type House struct {
	HouseRow
	OwnerUID string
	Storage  []StoredItem
	Created  time.Time
	Updated  time.Time
}

// GetHouse loads one house and what's stored in it.
func GetHouse(ctx context.Context, pool *pgxpool.Pool, id int64) (House, error) {
	var h House
	var storage []byte
	err := pool.QueryRow(ctx, `
		SELECT h.id, h.house_key, h.price, COALESCE(`+ownerExpr+`, ''), COALESCE(h.owner_player_id, 0), h.locked,
		       COALESCE(p.uid, ''), h.storage, h.created_at, h.updated_at
		FROM houses h LEFT JOIN players p ON p.id = h.owner_player_id
		WHERE h.id = $1`, id).Scan(&h.ID, &h.Key, &h.Price, &h.Owner, &h.OwnerID, &h.Locked, &h.OwnerUID, &storage, &h.Created, &h.Updated)
	if err != nil {
		return h, err
	}
	h.Name = gamenames.House(h.Key)
	h.Storage = parseStorage(storage)
	return h, nil
}

// parseStorage reads houses.storage ([[key, value], ...] pairs). The same
// trunk shape as vehicles (items / cash) is shown as items; anything else
// as key: value.
func parseStorage(raw []byte) []StoredItem {
	if items, cash := parseTrunk(raw); len(items) > 0 || cash > 0 {
		var out []StoredItem
		for _, s := range items {
			out = append(out, StoredItem{Name: s.Name, Value: "×" + strconv.FormatInt(s.Count, 10)})
		}
		if cash > 0 {
			out = append(out, StoredItem{Name: "Cash", Value: "$" + dollars(cash)})
		}
		return out
	}
	var pairs [][]json.RawMessage
	if json.Unmarshal(raw, &pairs) != nil {
		return nil
	}
	var out []StoredItem
	for _, p := range pairs {
		if len(p) < 2 {
			continue
		}
		var key string
		if json.Unmarshal(p[0], &key) != nil {
			continue
		}
		var n float64
		val := string(p[1])
		if json.Unmarshal(p[1], &n) == nil {
			val = "×" + strconv.FormatFloat(n, 'f', -1, 64)
		}
		out = append(out, StoredItem{Name: gamenames.Item(key), Value: val})
	}
	return out
}

// PlayerHouses lists the houses a player owns (Player Lookup).
func PlayerHouses(ctx context.Context, pool *pgxpool.Pool, playerID int64) ([]HouseRow, error) {
	rows, err := pool.Query(ctx, `SELECT id, house_key, price, locked FROM houses WHERE owner_player_id = $1 ORDER BY house_key`, playerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HouseRow
	for rows.Next() {
		h := HouseRow{OwnerID: playerID}
		if err := rows.Scan(&h.ID, &h.Key, &h.Price, &h.Locked); err != nil {
			return nil, err
		}
		h.Name = gamenames.House(h.Key)
		out = append(out, h)
	}
	return out, rows.Err()
}
