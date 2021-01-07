package crypto

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"filippo.io/edwards25519"
	"log"
)

func DeriveDHKey(privKey ed25519.PrivateKey, pubKey ed25519.PublicKey) []byte {
	var pub [32]byte

	copy(pub[:], pubKey[:32])

	// From https://cr.yp.to/ecdh.html
	scalar := GetScalar(privKey)
	secret := ScalarMult(scalar[:], pubKey)
	result := sha256.Sum256(secret[:])
	return result[:]
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

func ScalarMult(scalar, point []byte) []byte {
	uniformBytes := make([]byte, 64)
	copy(uniformBytes, scalar)
	log.Println("before: ", hex.EncodeToString(uniformBytes))
	scalarStruct := (&edwards25519.Scalar{}).SetUniformBytes(uniformBytes)
	log.Println("after: ", hex.EncodeToString(scalarStruct.Bytes()))
	pointResult, err := (&edwards25519.Point{}).SetBytes(point)
	if err != nil {
		log.Fatal(err)
	}
	return pointResult.ScalarMult(scalarStruct, pointResult).Bytes()
}
