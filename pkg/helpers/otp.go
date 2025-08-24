package helpers

import (
	"crypto/rand"
	"fmt"
)

// OTP helpers

// KeyLoginOTP is the Redis key for storing OTP codes for login
func KeyLoginOTP(uid string) string {
	return "login:otp:" + uid
}

// KeyTrustedDevice is the Redis key for storing trusted devices for a user
func KeyTrustedDevice(uid, dev string) string {
	return "login:trusted:" + uid + ":" + dev
}

// GenOTPCode generates a secure random 6-digit OTP code as a zero-padded string
func GenOTPCode() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// 6 digits: map random bytes to 000000-999999
	n := int(b[0])<<24 | int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	if n < 0 {
		n = -n
	}
	code := n % 1000000
	return fmt.Sprintf("%06d", code), nil
}
