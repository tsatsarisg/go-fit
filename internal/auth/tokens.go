package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"time"

	"github.com/tsatsarisg/go-fit/internal/user"
)

func GenerateToken(userID user.UserID, ttl time.Duration, scope string) (*Token, error) {
	token := &Token{
		UserID: userID,
		Expiry: time.Now().Add(ttl),
		Scope:  scope,
	}

	emptyBytes := make([]byte, 32)
	_, err := rand.Read(emptyBytes)
	if err != nil {
		return nil, err
	}

	token.Plaintext = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(emptyBytes)
	token.Hash = HashPlaintext(token.Plaintext)

	return token, nil
}

// HashPlaintext returns the sha256 digest of a token plaintext. Tokens are
// stored hashed so a DB compromise never yields usable bearer credentials;
// middleware uses this to look up tokens by hash on every authenticated request.
func HashPlaintext(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}
