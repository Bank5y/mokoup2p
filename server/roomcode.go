package server

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// roomCodeAlphabet is upper-case letters and digits without the easily
// confused 0, O, 1 and I: 32 symbols, so 8 of them give 40 bits.
const roomCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

const roomCodeLength = 8

// newRoomCode returns a random room code in its stored form (no separator).
func newRoomCode() (string, error) {
	var b strings.Builder
	max := big.NewInt(int64(len(roomCodeAlphabet)))
	for range roomCodeLength {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		b.WriteByte(roomCodeAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// normalizeRoomCode turns user input into the stored form, ignoring case,
// spaces and dashes. ok is false if the result cannot be a room code.
func normalizeRoomCode(input string) (code string, ok bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(input) {
		switch {
		case r == '-' || r == ' ':
		case strings.ContainsRune(roomCodeAlphabet, r):
			b.WriteRune(r)
		default:
			return "", false
		}
	}
	if b.Len() != roomCodeLength {
		return "", false
	}
	return b.String(), true
}

// displayRoomCode formats a stored room code as XXXX-XXXX.
func displayRoomCode(code string) string {
	return fmt.Sprintf("%s-%s", code[:4], code[4:])
}
