// Package budget tracks the execution ledger for a single run.
//
// Every action the kernel takes must be charged here before it is dispatched.
// A run stops when any single dimension is exhausted, and the planner uses
// Fraction() to decide when expensive actions stop being allowed.
package budget

import (
	"fmt"
	"sync"
)

// Budget is the ceiling for a run. A zero field means "unlimited".
type Budget struct {
	MaxActions int64
	MaxUSD     float64
	MaxWallMS  int64
	MaxTokens  int64
}

// Usage is the amount consumed so far.
type Usage struct {
	Actions int64
	USD     float64
	WallMS  int64
	Tokens  int64
}

// Add accumulates another charge.
func (u Usage) Add(o Usage) Usage {
	return Usage{
		Actions: u.Actions + o.Actions,
		USD:     u.USD + o.USD,
		WallMS:  u.WallMS + o.WallMS,
		Tokens:  u.Tokens + o.Tokens,
	}
}

// Ledger is a concurrency-safe budget accumulator.
type Ledger struct {
	mu sync.Mutex
	b  Budget
	u  Usage
}

func New(b Budget) *Ledger { return &Ledger{b: b} }

// Add charges the ledger and returns the new usage.
func (l *Ledger) Add(d Usage) Usage {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.u = l.u.Add(d)
	return l.u
}

func (l *Ledger) Usage() Usage {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.u
}

func (l *Ledger) Budget() Budget {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b
}

// Exceeded reports whether any dimension is spent, with a human reason.
func (l *Ledger) Exceeded() (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return exceeded(l.b, l.u)
}

func exceeded(b Budget, u Usage) (bool, string) {
	if b.MaxActions > 0 && u.Actions >= b.MaxActions {
		return true, fmt.Sprintf("action budget exhausted (%d/%d)", u.Actions, b.MaxActions)
	}
	if b.MaxUSD > 0 && u.USD >= b.MaxUSD {
		return true, fmt.Sprintf("dollar budget exhausted ($%.4f/$%.4f)", u.USD, b.MaxUSD)
	}
	if b.MaxWallMS > 0 && u.WallMS >= b.MaxWallMS {
		return true, fmt.Sprintf("wall-clock budget exhausted (%dms/%dms)", u.WallMS, b.MaxWallMS)
	}
	if b.MaxTokens > 0 && u.Tokens >= b.MaxTokens {
		return true, fmt.Sprintf("token budget exhausted (%d/%d)", u.Tokens, b.MaxTokens)
	}
	return false, ""
}

// Fraction is the largest consumed fraction across all bounded dimensions.
// Used by the policy engine: above ~0.75 the kernel stops allowing switches
// and expensive model calls.
func (l *Ledger) Fraction() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := 0.0
	if l.b.MaxActions > 0 {
		f = maxf(f, float64(l.u.Actions)/float64(l.b.MaxActions))
	}
	if l.b.MaxUSD > 0 {
		f = maxf(f, l.u.USD/l.b.MaxUSD)
	}
	if l.b.MaxWallMS > 0 {
		f = maxf(f, float64(l.u.WallMS)/float64(l.b.MaxWallMS))
	}
	if l.b.MaxTokens > 0 {
		f = maxf(f, float64(l.u.Tokens)/float64(l.b.MaxTokens))
	}
	return f
}

// CanAfford reports whether an incremental charge fits inside the budget.
func (l *Ledger) CanAfford(d Usage) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !wouldExceed(l.b, l.u, d)
}

func wouldExceed(b Budget, u, d Usage) bool {
	if b.MaxActions > 0 && u.Actions+d.Actions > b.MaxActions {
		return true
	}
	if b.MaxUSD > 0 && u.USD+d.USD > b.MaxUSD {
		return true
	}
	if b.MaxTokens > 0 && u.Tokens+d.Tokens > b.MaxTokens {
		return true
	}
	return false
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}