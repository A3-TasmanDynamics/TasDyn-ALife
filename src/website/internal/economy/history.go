package economy

// Economy history: the website records a snapshot of the money supply,
// outstanding bounties and market prices once an hour (economy_snapshots),
// and the Economy & Crime page charts them.

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"website/internal/gamenames"
)

// snapshotEvery is how often a snapshot is taken; a restart doesn't take
// an extra one if the last is newer than snapshotEvery minus a few minutes.
const snapshotEvery = time.Hour

// TakeSnapshot records one snapshot unless there's already a recent one.
// It reports whether it wrote one.
func TakeSnapshot(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	var recent bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM economy_snapshots WHERE taken_at > now() - make_interval(secs => $1))`,
		(snapshotEvery - 5*time.Minute).Seconds()).Scan(&recent); err != nil {
		return false, err
	}
	if recent {
		return false, nil
	}
	f, err := Supply(ctx, pool)
	if err != nil {
		return false, err
	}
	market := map[string]float64{}
	rows, err := pool.Query(ctx, `SELECT lower(item), price::float8 FROM market_prices`)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var k string
		var p float64
		if err := rows.Scan(&k, &p); err != nil {
			rows.Close()
			return false, err
		}
		market[k] = math.Round(p*100) / 100
	}
	rows.Close()
	mj, _ := json.Marshal(market)
	_, err = pool.Exec(ctx, `
		INSERT INTO economy_snapshots (civ_cash, cop_cash, medic_cash, civ_bank, cop_bank, medic_bank, bounties, market)
		VALUES ($1, $2, $3, $4, $5, $6, (SELECT COALESCE(sum(bounty_amount), 0) FROM wanted_crimes WHERE cleared_at IS NULL), $7::jsonb)`,
		f[0].Cash, f[1].Cash, f[2].Cash, f[0].Bank, f[1].Bank, f[2].Bank, string(mj))
	return err == nil, err
}

// RunSnapshots takes a snapshot now and then every hour until ctx ends.
func RunSnapshots(ctx context.Context, pool *pgxpool.Pool) {
	take := func() {
		if _, err := TakeSnapshot(ctx, pool); err != nil && ctx.Err() == nil {
			slog.Error("economy: snapshot failed (is database/fixes/2026-11-12_economy_snapshots.sql applied?)", "error", err)
		}
	}
	take()
	t := time.NewTicker(10 * time.Minute) // TakeSnapshot itself keeps it to one an hour
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			take()
		}
	}
}

// Point is one snapshot, ready to chart.
type Point struct {
	At       time.Time
	Cash     int64
	Bank     int64
	Bounties int64
	// MarketIndex is how far the market is from its base prices: the mean
	// of every CfgMarket item's (price - base) / base, in percent.
	MarketIndex float64
}

// History is every snapshot since `since`, oldest first.
func History(ctx context.Context, pool *pgxpool.Pool, since time.Time) ([]Point, error) {
	rows, err := pool.Query(ctx, `
		SELECT taken_at, civ_cash + cop_cash + medic_cash, civ_bank + cop_bank + medic_bank, bounties, market::text
		FROM economy_snapshots WHERE taken_at > $1 ORDER BY taken_at`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	base := gamenames.Market()
	var out []Point
	for rows.Next() {
		var p Point
		var mj string
		if err := rows.Scan(&p.At, &p.Cash, &p.Bank, &p.Bounties, &mj); err != nil {
			return nil, err
		}
		prices := map[string]float64{}
		_ = json.Unmarshal([]byte(mj), &prices)
		var sum float64
		n := 0
		for _, m := range base {
			if m.Buy <= 0 {
				continue
			}
			cur := float64(m.Buy)
			if v, ok := prices[m.Key]; ok {
				cur = v
			}
			sum += (cur - float64(m.Buy)) / float64(m.Buy) * 100
			n++
		}
		if n > 0 {
			p.MarketIndex = math.Round(sum/float64(n)*10) / 10
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
