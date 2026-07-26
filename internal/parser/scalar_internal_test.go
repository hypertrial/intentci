package parser

import "testing"

func TestScalarStringAllBranches(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"x", "x"},
		{true, "true"},
		{false, "false"},
		{int(7), "7"},
		{int64(8), "8"},
		{float64(9), "9"},
		{float64(1.25), "1.25"},
		{nil, ""},
		{struct{ A int }{1}, "{1}"},
	}
	for _, tc := range cases {
		if got := scalarString(tc.in); got != tc.want {
			t.Fatalf("%v: got %q want %q", tc.in, got, tc.want)
		}
	}
}
