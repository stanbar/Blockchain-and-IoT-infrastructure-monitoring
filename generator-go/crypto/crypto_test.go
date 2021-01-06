package crypto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"log"
	"testing"

	"github.com/stellar/go/keypair"
)

func TestDeriveDHKey(t *testing.T) {
	alice := keypair.MustRandom()
	bob := keypair.MustRandom()

	aPub, aPriv, err := ed25519.GenerateKey(nil)
	bPub, bPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println("aPub: ", hex.EncodeToString(aPub))
	log.Println("aPriv: ", hex.EncodeToString(aPriv))
	log.Println("bPub: ", hex.EncodeToString(bPub))
	log.Println("bPriv: ", hex.EncodeToString(bPriv))
	derivedByA := DeriveDHKey(aPriv, bPub)
	derivedByB := DeriveDHKey(bPriv, aPub)
	derivedByAString := hex.EncodeToString(derivedByA)
	derivedByBString := hex.EncodeToString(derivedByB)
	log.Println("Derived by A: ", derivedByAString)
	log.Println("Derived by B: ", derivedByBString)
	if !bytes.Equal(derivedByA, derivedByB) {
		t.Error("keys does not match")
	}
}
