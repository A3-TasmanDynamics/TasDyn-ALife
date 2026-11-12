// Package gamenames turns the game's keys into the names players see: a
// vehicle classname ("C_Offroad_01_F" -> "Offroad"), a licence key
// ("driver" -> "Driver Licence") or a virtual item ("copper_ore" ->
// "Copper Ore"). The tables are generated from the mission config
// (names_gen.go, tools/config_names/website_names.py); anything not in
// them falls back to a tidied-up key rather than nothing.
package gamenames

import "strings"

func Vehicle(class string) string { return lookup(vehicles, class) }
func Licence(key string) string   { return lookup(licences, key) }
func Item(key string) string      { return lookup(items, key) }

func Crime(key string) string { return lookup(crimes, key) }

// Gear names an Arma class (weapon, clothing, attachment, magazine, item)
// by Arma's own name when the mission's config mentions it; otherwise the
// classname as it is.
func Gear(class string) string {
	if n, ok := gear[strings.ToLower(strings.TrimSpace(class))]; ok {
		return n
	}
	return class
}

// LicenceDef is one CfgLicences entry and the faction it's for ("civ",
// "cop", "medic").
type LicenceDef struct {
	Key  string
	Name string
	Side string
}

// Licences lists every licence for a faction, in config order.
func Licences(side string) []LicenceDef {
	var out []LicenceDef
	for _, l := range licenceList {
		if l.Side == side {
			out = append(out, l)
		}
	}
	return out
}

// IsLicence reports whether key is a CfgLicences class for that faction.
func IsLicence(side, key string) bool {
	for _, l := range licenceList {
		if l.Side == side && l.Key == key {
			return true
		}
	}
	return false
}

// House names a house by its key (houses.house_key): there's no house
// config yet, so it's the tidied-up key.
func House(key string) string { return humanize(key) }

// VehiclesNamed is the classnames (lower case) whose name contains q, so
// "hatchback" finds C_Hatchback_01_F. Nothing for an empty q.
func VehiclesNamed(q string) []string {
	q = strings.ToLower(strings.TrimSpace(q))
	out := []string{}
	if q == "" {
		return out
	}
	for class, name := range vehicles {
		if strings.Contains(strings.ToLower(name), q) {
			out = append(out, class)
		}
	}
	return out
}

// Any names a class of any kind -- virtual item, vehicle or Arma gear --
// and says whether it's known; unknown classes come back as they are.
func Any(class string) (string, bool) {
	k := strings.ToLower(strings.TrimSpace(class))
	for _, m := range []map[string]string{items, vehicles, gear} {
		if n, ok := m[k]; ok {
			return n, true
		}
	}
	return class, false
}

func lookup(m map[string]string, key string) string {
	if n, ok := m[strings.ToLower(strings.TrimSpace(key))]; ok {
		return n
	}
	return humanize(key)
}

// humanize turns "kavala_townhouse" or "C_Offroad_01_F" into
// "Kavala townhouse" / "Offroad 01".
func humanize(key string) string {
	k := strings.TrimSpace(key)
	for _, p := range []string{"C_", "B_", "O_", "I_", "license_", "licence_"} {
		k = strings.TrimPrefix(k, p)
	}
	k = strings.TrimSuffix(k, "_F")
	k = strings.ReplaceAll(k, "_", " ")
	if k == "" {
		return key
	}
	return strings.ToUpper(k[:1]) + k[1:]
}

// MarketBase is one CfgMarket item's base prices (dollars): what the
// dynamic market drifts back toward, and the range its buy price stays in.
type MarketBase struct {
	Key       string
	Buy, Sell int64
	Min, Max  int64
}

// Market lists every market item's base prices, in config order.
func Market() []MarketBase { return marketList }
