package utils

import (
	"encoding/hex"
	"errors"
	"golang.org/x/crypto/bcrypt"
	"math/rand"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(bytes), err
}

func ComparePasswords(hashedPassword string, plainPassword string) error {

	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))

}

func GenerateSecureToken(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("invalid token length")
	}

	// Create a byte slice of the required length
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Convert the byte slice to a hexadecimal string
	return hex.EncodeToString(bytes), nil
}

func GenerateOtpCode(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("invalid OTP length")
	}

	const digits = "0123456789"
	otp := make([]byte, length)
	for i := range otp {
		otp[i] = digits[rand.Intn(len(digits))]
	}

	return string(otp), nil
}
