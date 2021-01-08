package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"github.com/jorrizza/ed2curve25519"
	"golang.org/x/crypto/curve25519"
)

func DeriveDHKey(privKey ed25519.PrivateKey, pubKey ed25519.PublicKey) ([]byte, error) {
	scalar := ed2curve25519.Ed25519PrivateKeyToCurve25519(privKey)
	curvePub := ed2curve25519.Ed25519PublicKeyToCurve25519(pubKey)

	secret, err := scalarMult(scalar, curvePub)
	if err != nil {
		return nil, err
	}
	result := sha256.Sum256(secret[:])
	return result[:], nil
}

func scalarMult(scalar, point []byte) ([]byte, error) {
	return curve25519.X25519(scalar, point)
}
