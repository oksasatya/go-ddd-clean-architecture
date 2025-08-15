package helpers

import "golang.org/x/crypto/bcrypt"

// HashPassword hashes the plain text password using bcrypt
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CompareHashAndPassword compares a bcrypt hash with a plain password
func CompareHashAndPassword(hash string, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
