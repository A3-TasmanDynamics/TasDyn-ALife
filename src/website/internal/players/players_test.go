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

func TestLicenceNames(t *testing.T) {
	got := licenceNames([]byte(`["driver","boat","driver","some_new_one"]`))
	if want := []string{"Driver Licence", "Boating Licence", "Some new one"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := licenceNames([]byte(`not json`)); got != nil {
		t.Errorf("not json: got %v", got)
	}
}
