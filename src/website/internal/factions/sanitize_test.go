package factions

import (
	"strings"
	"testing"
)

func TestSanitizeDocEditorStyles(t *testing.T) {
	in := `<h1>T</h1><p class="doc-subtitle">Sub</p><p class="evil">x</p>` +
		`<p><span style="color: rgb(248, 113, 113)">red</span><span style="background-color: #7f1d1d">hl</span>` +
		`<span style="position: fixed; color: red">bad</span></p><script>alert(1)</script>`
	out := SanitizeDoc(in)
	for _, want := range []string{`class="doc-subtitle"`, `color: rgb(248, 113, 113)`, `background-color: #7f1d1d`} {
		if !strings.Contains(out, want) {
			t.Errorf("lost %q: %s", want, out)
		}
	}
	for _, bad := range []string{"evil", "position", "script", "color: red"} {
		if strings.Contains(out, bad) {
			t.Errorf("kept %q: %s", bad, out)
		}
	}
}
