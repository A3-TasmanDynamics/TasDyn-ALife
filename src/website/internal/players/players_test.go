package players

import (
	"reflect"
	"testing"
)

func TestDollars(t *testing.T) {
	cases := map[int64]string{
		0: "0", 5: "0.05", 100: "1", 150000: "1,500", 123456789: "1,234,567.89", -250: "-2.50",
	}
	for cents, want := range cases {
		if got := dollars(cents); got != want {
			t.Errorf("dollars(%d) = %q, want %q", cents, got, want)
		}
	}
}

func TestMergeLicences(t *testing.T) {
	got := mergeLicences([]byte(`["driver","boat"]`), []byte(`["driver","pilot"]`), []byte(`not json`), nil)
	if want := []string{"driver", "boat", "pilot"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
