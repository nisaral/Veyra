package budget

import "testing"

func TestExceededOnEachDimension(t *testing.T) {
	cases := []struct {
		name string
		b    Budget
		u    Usage
		want bool
	}{
		{"actions", Budget{MaxActions: 5}, Usage{Actions: 5}, true},
		{"usd", Budget{MaxUSD: 0.10}, Usage{USD: 0.10}, true},
		{"wall", Budget{MaxWallMS: 1000}, Usage{WallMS: 1000}, true},
		{"tokens", Budget{MaxTokens: 100}, Usage{Tokens: 100}, true},
		{"under", Budget{MaxUSD: 1}, Usage{USD: 0.5}, false},
		{"unbounded", Budget{}, Usage{Actions: 1e6}, false},
	}
	for _, c := range cases {
		if got, _ := Exceeded(c.b, c.u); got != c.want {
			t.Errorf("%s: Exceeded=%v want %v", c.name, got, c.want)
		}
	}
}

func TestFractionIsTheTightestBound(t *testing.T) {
	b := Budget{MaxActions: 10, MaxUSD: 1.0}
	u := Usage{Actions: 5, USD: 0.9}
	if got := Fraction(b, u); got < 0.89 || got > 0.91 {
		t.Fatalf("fraction should follow the tightest dimension, got %f", got)
	}
}