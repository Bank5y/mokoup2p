package server

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Room subnets are /24s drawn at random from 10.208.0.0/12 (4096 of them).
// That block is private (RFC 1918) and stays clear of the ranges home and
// office routers commonly hand out (192.168.0.0/16, 10.0.0.0/16, 10.1.0.0/16,
// 172.16.0.0/12 including Docker's 172.17.0.0/16) and of EasyTier's default
// 10.144.144.0/24. Uniqueness across rooms is enforced by the database.
const (
	roomSubnetBase  = 208 // second octet of the pool's first /24
	roomSubnetCount = 16 * 256
)

func newRoomSubnet() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(roomSubnetCount))
	if err != nil {
		return "", err
	}
	i := int(n.Int64())
	return fmt.Sprintf("10.%d.%d.0/24", roomSubnetBase+i/256, i%256), nil
}
