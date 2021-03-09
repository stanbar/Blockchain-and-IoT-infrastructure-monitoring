package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/sha256"
	"math/big"

	"github.com/jorrizza/ed2curve25519"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/strkey"
	"github.com/stellot/stellot-iot/pkg/utils"
	"golang.org/x/crypto/curve25519"
)

func StellarAddressToPubKey(address string) ed25519.PublicKey {
	return ed25519.PublicKey(strkey.MustDecode(strkey.VersionByteAccountID, address))
}

func StellarKeypairToPrivKey(kp *keypair.Full) ed25519.PrivateKey {
	rawSeed := strkey.MustDecode(strkey.VersionByteSeed, kp.Seed())
	reader := bytes.NewReader(rawSeed)
	_, priv, err := ed25519.GenerateKey(reader)
	if err != nil {
		panic(err)
	}
	return priv
}

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

func EncryptToMemo(seqNumber int64, kp *keypair.Full, to string, log [32]byte) (*[32]byte, error) {
	defer utils.Duration(utils.Track("En/decrypt to memo"))
	var payload []byte
	copy(payload[:], log[:32])

	pubKey := StellarAddressToPubKey(to)
	privKey := StellarKeypairToPrivKey(kp)

	ecdhKey, err := DeriveDHKey(privKey, pubKey)
	if err != nil {
		return nil, err
	}

	result, err := encrypt(seqNumber, ecdhKey, log)
	return result, err
}

func encrypt(seqNumber int64, ecdhKey []byte, msg [32]byte) (*[32]byte, error) {
	block, err := aes.NewCipher(ecdhKey)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+32)
	iv := ciphertext[:aes.BlockSize]
	new(big.Int).Mul(big.NewInt(int64(seqNumber)), big.NewInt(2)).FillBytes(iv[8:])

	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], msg[:])

	var out [32]byte
	copy(out[:], ciphertext[aes.BlockSize:])

	return &out, nil
}
