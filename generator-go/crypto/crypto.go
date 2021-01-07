package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"golang.org/x/crypto/curve25519"
)

func DeriveDHKey(privKey ed25519.PrivateKey, pubKey ed25519.PublicKey) ([]byte, error) {
	var pub [32]byte

	copy(pub[:], pubKey[:32])

	// From https://cr.yp.to/ecdh.html
	scalar := GetScalar(privKey)
	secret, err := ScalarMult(scalar[:], pubKey)
	if err != nil {
		return nil, err
	}
	result := sha256.Sum256(secret[:])
	return result[:], err
}

func GetScalar(privKey []byte) [32]byte {
	var scalar [32]byte
	// From https://cr.yp.to/ecdh.html
	hash := CalculateHash(privKey)
	BytesOperations(&hash)
	copy(scalar[:], hash[:32])
	return scalar
}

func CalculateHash(privKey ed25519.PrivateKey) [64]byte {
	return sha512.Sum512(privKey[:32])
}

func BytesOperations(bytes *[64]byte) {
	bytes[0] &= 0xf8
	bytes[31] &= 0x3f
	bytes[31] |= 0x40
}

func ScalarMult(scalar, point []byte) ([]byte, error) {
	return curve25519.X25519(scalar, point)
}
