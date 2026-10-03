package runstore

import (
	veyrav1 "github.com/nisaral/veyra/gen/veyra/v1"
	"github.com/nisaral/veyra/internal/budget"
)

// Charge accumulates usage against the run's ledger.
func (r *Run) Charge(u budget.Usage) {
	r.mu.Lock()
	r.Usage = r.Usage.Add(u)
	r.mu.Unlock()
}

// Exceeded reports whether any budget dimension is spent.
func (r *Run) Exceeded() (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return budget.Exceeded(r.Budget, r.Usage)
}

// Fraction is the consumed fraction of the tightest bound.
func (r *Run) Fraction() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return budget.Fraction(r.Budget, r.Usage)
}

// RemainingUSD is the unspent dollar budget (0 when unbounded).
func (r *Run) RemainingUSD() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return budget.RemainingUSD(r.Budget, r.Usage)
}

// BudgetProto returns the run budget as a wire message.
func (r *Run) BudgetProto() *veyrav1.Budget {
	r.mu.Lock()
	defer r.mu.Unlock()
	return budgetProto(r.Budget)
}

// UsageProto returns the run usage as a wire message.
func (r *Run) UsageProto() *veyrav1.BudgetUsage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return usageProto(r.Usage)
}