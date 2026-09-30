package handlers

import (
	"log/slog"
	"net/http"
	"strings"
)

// ItemPrices is Admin → Item Prices (GAMEPANEL_PARITY §5.5): read-only,
// as a reference for compensation. The mission has no economy config yet
// (Phase 4 decides whether prices live in the DB or the mission), so the
// only prices that exist today are house prices, which are in the DB.

type housePrice struct {
	Key   string
	Price int64
	Owner string
}

type pricesData struct {
	Base
	AdminShell
	Q      string
	Houses []housePrice
	Owned  int
}

func (d *Deps) ItemPrices(w http.ResponseWriter, r *http.Request) {
	data := pricesData{Base: baseFrom(r, "Item Prices"), AdminShell: d.adminShell(r, "prices"), Q: strings.TrimSpace(r.URL.Query().Get("q"))}
	rows, err := d.Pool.Query(r.Context(), `
		SELECT h.house_key, h.price, COALESCE(p.name, '')
		FROM houses h LEFT JOIN players p ON p.id = h.owner_player_id
		WHERE $1 = '' OR h.house_key ILIKE '%' || $1 || '%'
		ORDER BY h.price DESC, h.house_key
		LIMIT 500`, data.Q)
	if err != nil {
		slog.Error("item prices: query failed", "error", err)
		data.Error = "Couldn't load prices."
	} else {
		defer rows.Close()
		for rows.Next() {
			var h housePrice
			if err := rows.Scan(&h.Key, &h.Price, &h.Owner); err != nil {
				slog.Error("item prices: scan failed", "error", err)
				break
			}
			if h.Owner != "" {
				data.Owned++
			}
			data.Houses = append(data.Houses, h)
		}
	}
	d.Render.Render(w, "prices.html", data)
}
