package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"website/internal/chart"
	"website/internal/economy"
	"website/internal/events"
)

type economyData struct {
	Base
	AdminShell
	Window  string // 24h / 7d / 30d
	Windows []events.Range

	Supply      []economy.Faction
	SupplyCash  int64
	SupplyBank  int64
	In, Out     []economy.Flow
	InTotal     int64
	OutTotal    int64
	Rich        []economy.Rich
	Market      []economy.Price
	Crime       economy.Crime
	Robberies   []events.Event
	Vault       economy.Vault
	VaultCounts [2]int64 // gear pieces, virtual items

	// History (economy_snapshots, one an hour): charts and their table.
	History     []economy.Point
	BankChart   chart.Chart
	CashChart   chart.Chart
	MarketChart chart.Chart
	Daily       []economy.Point // the last snapshot of each day, newest first
}

// Net is what the window added to (or took from) the money supply.
func (d economyData) Net() int64 { return d.InTotal - d.OutTotal }

// NetAbs is Net without its sign (the template adds + / -).
func (d economyData) NetAbs() int64 {
	if n := d.Net(); n < 0 {
		return -n
	}
	return d.Net()
}

func (d economyData) SupplyTotal() int64 { return d.SupplyCash + d.SupplyBank }

// Pct is part as a whole-number percentage of total (0 when total is 0).
func (d economyData) Pct(part, total int64) int64 {
	if total <= 0 {
		return 0
	}
	return part * 100 / total
}

// Economy is /admin/economy: the Economy & Crime overview.
func (d *Deps) Economy(w http.ResponseWriter, r *http.Request) {
	data := d.economyPage(r.Context(), r.URL.Query().Get("range"))
	data.Base, data.AdminShell = baseFrom(r, "Economy & Crime"), d.adminShell(r, "economy")
	d.Render.Render(w, "economy.html", data)
}

// economyPage gathers the page for a window (24h / 7d / 30d; anything else
// is 7d). A section that fails to load is logged and left empty.
func (d *Deps) economyPage(ctx context.Context, window string) economyData {
	data := economyData{Window: window}
	back := 7 * 24 * time.Hour
	for _, rg := range events.Ranges {
		if rg.Back > 0 {
			data.Windows = append(data.Windows, rg)
			if rg.Key == data.Window {
				back = rg.Back
			}
		}
	}
	if back == 7*24*time.Hour {
		data.Window = "7d"
	}
	since := time.Now().Add(-back)
	logErr := func(what string, err error) {
		if err != nil {
			slog.Error("economy page: "+what+" failed", "error", err)
		}
	}
	var err error
	data.Supply, err = economy.Supply(ctx, d.Pool)
	logErr("money supply", err)
	for _, f := range data.Supply {
		data.SupplyCash += f.Cash
		data.SupplyBank += f.Bank
	}
	data.In, data.Out, err = economy.Flows(ctx, d.Pool, since)
	logErr("money flows", err)
	data.InTotal, data.OutTotal = economy.Total(data.In), economy.Total(data.Out)
	data.Rich, err = economy.Richest(ctx, d.Pool, 10)
	logErr("rich list", err)
	data.Market, err = economy.Market(ctx, d.Pool)
	logErr("market", err)
	data.Crime, err = economy.CrimeStats(ctx, d.Pool, since)
	logErr("crime", err)
	data.Robberies, _, err = events.List(ctx, d.Pool, events.Filter{Category: "crime", From: since, Limit: 10})
	logErr("recent crime", err)
	data.Vault, err = economy.EvidenceVault(ctx, d.Pool)
	logErr("evidence vault", err)
	for _, g := range data.Vault.Gear {
		data.VaultCounts[0] += g.N
	}
	for _, it := range data.Vault.Items {
		data.VaultCounts[1] += it.N
	}
	data.History, err = economy.History(ctx, d.Pool, since)
	logErr("history", err)
	if len(data.History) >= 2 {
		times := make([]time.Time, len(data.History))
		cash := make([]float64, len(data.History))
		bank := make([]float64, len(data.History))
		idx := make([]float64, len(data.History))
		for i, p := range data.History {
			times[i], cash[i], bank[i], idx[i] = p.At, float64(p.Cash), float64(p.Bank), p.MarketIndex
		}
		// Separate charts: bank is usually many times cash, which would flatten it.
		data.BankChart = chart.Build(times, []chart.Series{{Name: "Bank", Class: "lc-bank", Values: bank}}, chart.Money, true)
		data.CashChart = chart.Build(times, []chart.Series{{Name: "Cash", Class: "lc-cash", Values: cash}}, chart.Money, true)
		data.MarketChart = chart.Build(times, []chart.Series{{Name: "Market", Class: "lc-market", Values: idx}}, chart.Percent, false)
	}
	for i := len(data.History) - 1; i >= 0; i-- {
		p := data.History[i]
		if n := len(data.Daily); n == 0 || data.Daily[n-1].At.Format("2006-01-02") != p.At.Format("2006-01-02") {
			data.Daily = append(data.Daily, p)
		}
	}
	return data
}
