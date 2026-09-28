package handlers

import "testing"

func TestCSVSafe(t *testing.T) {
	cases := map[string]string{
		"=cmd|' /c calc'!A0": "'=cmd|' /c calc'!A0",
		"+1":                 "'+1",
		"-2":                 "'-2",
		"@SUM(A1)":           "'@SUM(A1)",
		"\tx":                "'\tx",
		"normal name":        "normal name",
		"":                   "",
	}
	for in, want := range cases {
		if got := csvSafe(in); got != want {
			t.Errorf("csvSafe(%q) = %q, want %q", in, got, want)
		}
	}
}
