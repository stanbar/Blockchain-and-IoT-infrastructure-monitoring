package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"github.com/jorrizza/ed2curve25519"
	"golang.org/x/crypto/curve25519"
	"log"
)

func DeriveDHKey(privKey ed25519.PrivateKey, pubKey ed25519.PublicKey) []byte {
	scalar := GetScalar(privKey)
	curvePub := GetPubKey(pubKey)

	secret, err := ScalarMult(scalar, curvePub)
	if err != nil {
		log.Fatal(err)
	}
	result := sha256.Sum256(secret[:])
	return result[:]
}

func GetScalar(privKey ed25519.PrivateKey) []byte {
	return ed2curve25519.Ed25519PrivateKeyToCurve25519(privKey)
}

func GetPubKey(pubKey ed25519.PublicKey) []byte {
	return ed2curve25519.Ed25519PublicKeyToCurve25519(pubKey)
}

func ScalarMult(scalar, point []byte) ([]byte, error) {
	return curve25519.X25519(scalar, point)
}
