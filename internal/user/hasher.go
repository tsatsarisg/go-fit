package user

import "golang.org/x/crypto/bcrypt"

// Hasher is the strategy used by Service to hash new passwords. Decoupled
// from the bcrypt impl so tests can inject a cheap-cost variant (integration
// tests that churn through Register hit Service.Register dozens of times;
// DefaultCost at ~100ms/hash makes suites minute-slow) and so a future
// migration to argon2 is a drop-in at the app wiring site, not a surgery
// on every call site.
type Hasher interface {
	Hash(plaintext string) (password, error)
}

// BcryptHasher is the production Hasher. Cost is bcrypt work factor; leave at
// bcrypt.DefaultCost (currently 10) for prod, bcrypt.MinCost (4) for tests.
type BcryptHasher struct {
	Cost int
}

// NewBcryptHasher returns a hasher at the given cost. If cost is zero or
// outside bcrypt's accepted range, bcrypt.DefaultCost is used so a mis-wired
// app doesn't silently produce unhashed-weak credentials.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{Cost: cost}
}

func (h *BcryptHasher) Hash(plaintext string) (password, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintext), h.Cost)
	if err != nil {
		return password{}, err
	}
	return password{hash: hash}, nil
}
