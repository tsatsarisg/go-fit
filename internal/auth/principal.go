package auth

import "github.com/tsatsarisg/go-fit/internal/user"

// Principal is the identity that authenticated code branches act on. It is
// intentionally a minimal projection of the user (ID + Username) so transport
// and feature packages never carry the full *user.User aggregate through
// request context — that was the A3 leak in the original layout.
type Principal struct {
	ID       user.UserID
	Username string
}

// AnonymousPrincipal represents a request with no (or an invalid) bearer
// token. Middleware always stores a non-nil *Principal in context, falling
// back to this value, so handlers can call IsAnonymous without nil checks.
var AnonymousPrincipal = &Principal{}

// IsAnonymous reports whether p is the sentinel AnonymousPrincipal. Compared
// by pointer identity so a zero-valued Principal constructed elsewhere is
// not silently treated as anonymous.
func (p *Principal) IsAnonymous() bool {
	return p == AnonymousPrincipal
}
