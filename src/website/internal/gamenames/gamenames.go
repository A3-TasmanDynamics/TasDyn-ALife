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
