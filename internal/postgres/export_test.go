package postgres

// The names that the tests of the package postgres_test use.

// ClaimSQL is the SQL of a claim of the broker.
const ClaimSQL = claimSQL

// SetClaim changes the SQL of the claims of the broker: a stub check gives a
// claim without its row lock.
func SetClaim(b *Broker, sql string) { b.claim = sql }

// Options gives the times of the broker after the defaults.
func Options(b *Broker) BrokerOptions { return b.opt }
