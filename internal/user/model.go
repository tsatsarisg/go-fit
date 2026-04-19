package user

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// password wraps a bcrypt hash so the plaintext never leaks into logs or JSON.
type password struct {
	hash []byte
}

func (p *password) Set(plainText string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainText), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	p.hash = hash
	return nil
}

func (p *password) Matches(plainText string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plainText))

	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	PasswordHash password  `json:"-"`
	Bio          string    `json:"bio,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var AnonymousUser = &User{}

func (u *User) IsAnonymous() bool {
	return u == AnonymousUser
}

var dummyPasswordHash = func() []byte {
	h, err := bcrypt.GenerateFromPassword([]byte("timing-equalization-dummy"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return h
}()

// VerifyPassword runs bcrypt against the user's hash, or against a fixed dummy
// hash when user is nil, so response timing does not reveal whether the
// username existed. Returns (true, nil) only on a real match.
func VerifyPassword(user *User, plainText string) (bool, error) {
	hash := dummyPasswordHash
	if user != nil {
		hash = user.PasswordHash.hash
	}
	err := bcrypt.CompareHashAndPassword(hash, []byte(plainText))
	if err == bcrypt.ErrMismatchedHashAndPassword {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return user != nil, nil
}
