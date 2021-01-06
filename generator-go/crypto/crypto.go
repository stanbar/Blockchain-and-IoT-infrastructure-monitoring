package crypto

import (
	"crypto/ed25519"
	"golang.org/x/crypto/curve25519"
)

func DeriveDHKey(privKey ed25519.PrivateKey, pubKey ed25519.PublicKey) []byte {
	var priv, pub, secret [32]byte

	copy(priv[:], privKey[:32])
	copy(pub[:], pubKey[:32])
	secret = [32]byte{}

	curve25519.ScalarMult(&secret, &priv, &pub)
	return secret[:]
}
