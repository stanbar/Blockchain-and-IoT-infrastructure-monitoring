package crypto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"log"
	"testing"

	"github.com/stellar/go/keypair"
	"github.com/stellar/go/strkey"
)

func keys(kp *keypair.Full) (ed25519.PublicKey, ed25519.PrivateKey) {
	rawSeed := strkey.MustDecode(strkey.VersionByteSeed, kp.Seed())
	reader := bytes.NewReader(rawSeed)
	pub, priv, err := ed25519.GenerateKey(reader)
	if err != nil {
		panic(err)
	}
	return pub, priv
}

func TestDeriveDHKey(t *testing.T) {
	alice, err := keypair.ParseFull("SACSBRUH43EU4YBHK2UT4WOQKPIE4HLZPQBOIES7ZOAVMVCW5ZQRGVUZ")
	if err != nil {
		t.Error(err)
	}
	bob, err := keypair.ParseFull("SAAQ3OASEUGQQX6CRAWJTPZFAM6W2YOYBRIRNIYRUQY3QGGLFN4XYVRF")
	if err != nil {
		t.Error(err)
	}

	aPub, aPriv := keys(alice)
	bPub, bPriv := keys(bob)

	log.Println("aPub: ", hex.EncodeToString(aPub))
	log.Println("aPriv: ", hex.EncodeToString(aPriv))
	log.Println("bPub: ", hex.EncodeToString(bPub))
	log.Println("bPriv: ", hex.EncodeToString(bPriv))

	hash := CalculateHash(bPriv)
	if hex.EncodeToString(hash[:]) != "7d26df77f87437f12e910b4a440d02227d0c5f28bd328533fa139cb9fe1742f8383a8722db43cf18187b3a6ffdb23d1ace65a5d270cc4a40d1ee65d464e986ae" {
		t.Error("Hashes does not mach")
	}

	BytesOperations(&hash)
	if hex.EncodeToString(hash[:]) != "7826df77f87437f12e910b4a440d02227d0c5f28bd328533fa139cb9fe174278383a8722db43cf18187b3a6ffdb23d1ace65a5d270cc4a40d1ee65d464e986ae" {
		t.Error("Bytes operations failed")
	}

	scalar := hash[:32]
	log.Println("scalar", hex.EncodeToString(scalar[:]))
	// scalar: '7826df77f87437f12e910b4a440d02227d0c5f28bd328533fa139cb9fe174278'
	if hex.EncodeToString(hash[:32]) != "7826df77f87437f12e910b4a440d02227d0c5f28bd328533fa139cb9fe174278" {
		t.Error("Get Scalar does not match")
	}

	secret, err := ScalarMult(hash[:32], aPub)
	if err != nil {
		t.Error("Error", err)
	}

	log.Println("secret", hex.EncodeToString(secret[:]))
	if hex.EncodeToString(secret[:]) != "bbeafe8cf8912ea4c8e49d4c53344ce5caaf9f9fb4c2501971d78b9450b5bc92" {
		t.Error("Multiplication failed")
	}

	derivedByA, err := DeriveDHKey(aPriv, bPub)
	if err != nil {
		t.Error(err)
	}
	derivedByB, err := DeriveDHKey(bPriv, aPub)
	if err != nil {
		t.Error(err)
	}
	derivedByAString := hex.EncodeToString(derivedByA)
	derivedByBString := hex.EncodeToString(derivedByB)
	log.Println("Derived by A: ", derivedByAString)
	log.Println("Derived by B: ", derivedByBString)
	if !bytes.Equal(derivedByA, derivedByB) {
		t.Error("keys does not match")
	}
}
