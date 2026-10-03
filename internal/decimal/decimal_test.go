package decimal

import "testing"

func TestExact(t *testing.T) {
	for _, test := range []struct{ input, want string }{{"01.2300", "1.23"}, {"1e-3", "0.001"}, {"1.234e3", "1234"}, {"0e10", "0"}, {".0012300e3", "1.23"}, {"1000e-3", "1"}, {"+001e-3", "0.001"}} {
		got, err := Normalize(test.input)
		if err != nil || got != test.want {
			t.Fatalf("%s => %s %v", test.input, got, err)
		}
	}
	got, err := Median([]string{"1", "2", "3", "4"})
	if err != nil || got != "2.5" {
		t.Fatal(got, err)
	}
	got, err = Median([]string{"0.1", "0.2"})
	if err != nil || got != "0.15" {
		t.Fatal(got, err)
	}
}
func TestInvalid(t *testing.T) {
	for _, s := range []string{"NaN", "Inf", "-1", "-0", "1/2", "1e10001", "1e5000"} {
		if _, err := Normalize(s); err == nil {
			t.Fatal("accepted", s)
		}
	}
}
