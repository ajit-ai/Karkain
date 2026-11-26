package sema

import "testing"

func TestTypeValue_ParseRoundTrip(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"int", "int"},
		{"float64", "float64"},
		{"bool", "bool"},
		{"string", "string"},
		{"int[]", "int[]"},
		{"Option", "Option"},
	}
	for _, c := range cases {
		got := ParseTypeString(c.in)
		if got == nil || got.String() != c.want {
			t.Errorf("ParseTypeString(%q) = %v, want %q", c.in, got, c.want)
		}
	}
}

func TestTypeValue_Equal(t *testing.T) {
	a := ParseTypeString("int[]")
	b := ParseTypeString("int[]")
	if !a.Equal(b) {
		t.Fatal("equal types reported unequal")
	}
	c := ParseTypeString("string[]")
	if a.Equal(c) {
		t.Fatal("distinct types reported equal")
	}
}