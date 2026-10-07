package budget

// Exceeded reports whether the budget is spent, with a human-readable reason.
func Exceeded(b Budget, u Usage) (bool, string) { return exceeded(b, u) }

// Fraction is the largest consumed fraction across bounded dimensions.
func Fraction(b Budget, u Usage) float64 {
	f := 0.0
	if b.MaxActions > 0 {
		f = maxf(f, float64(u.Actions)/float64(b.MaxActions))
	}
	if b.MaxUSD > 0 {
		f = maxf(f, u.USD/b.MaxUSD)
	}
	if b.MaxWallMS > 0 {
		f = maxf(f, float64(u.WallMS)/float64(b.MaxWallMS))
	}
	if b.MaxTokens > 0 {
		f = maxf(f, float64(u.Tokens)/float64(b.MaxTokens))
	}
	return f
}

// RemainingUSD returns the unspent dollar budget (0 when unbounded).
func RemainingUSD(b Budget, u Usage) float64 {
	if b.MaxUSD <= 0 {
		return 0
	}
	if r := b.MaxUSD - u.USD; r > 0 {
		return r
	}
	return 0
}