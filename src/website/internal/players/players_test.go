package players

import (
	"reflect"
	"testing"
)

func TestDollars(t *testing.T) {
	cases := map[int64]string{
		0: "0", 5: "5", 999: "999", 1500: "1,500", 1234567: "1,234,567", -2500: "-2,500",
	}
	for amount, want := range cases {
		if got := Dollars(amount); got != want {
			t.Errorf("Dollars(%d) = %q, want %q", amount, got, want)
		}
	}
}

func TestMergeLicences(t *testing.T) {
	got := mergeLicences([]byte(`["driver","boat"]`), []byte(`["driver","pilot"]`), []byte(`not json`), nil)
	if want := []string{"driver", "boat", "pilot"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
