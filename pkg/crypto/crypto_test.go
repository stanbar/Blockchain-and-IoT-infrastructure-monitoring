package crypto

import (
	"bytes"
	"crypto/ed25519"
	"fmt"
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

func TestDeriveDHKeyRandom(t *testing.T) {
	for i := 1; i < 10; i++ {
		alice, err := keypair.Random()
		if err != nil {
			t.Error(err)
		}
		bob, err := keypair.Random()
		if err != nil {
			t.Error(err)
		}

		aPub, aPriv := keys(alice)
		bPub, bPriv := keys(bob)

		derivedByA, err := DeriveDHKey(aPriv, bPub)
		if err != nil {
			t.Error(err)
		}
		derivedByB, err := DeriveDHKey(bPriv, aPub)
		if err != nil {
			t.Error(err)
		}
		if !bytes.Equal(derivedByA, derivedByB) {
			t.Error("keys does not match")
		}
	}
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

	derivedByA, err := DeriveDHKey(aPriv, bPub)
	if err != nil {
		t.Error(err)
	}
	derivedByB, err := DeriveDHKey(bPriv, aPub)
	if err != nil {
		t.Error(err)
	}
	if !bytes.Equal(derivedByA, derivedByB) {
		t.Error("keys does not match")
	}
}

func TestEncryptMemo(t *testing.T) {
	alice, err := keypair.Random()
	if err != nil {
		t.Error(err)
	}
	bob, err := keypair.Random()
	if err != nil {
		t.Error(err)
	}
	plaintext := [32]byte{97, 98, 99} // "abc"
	fmt.Println("plaintext", plaintext, string(plaintext[:]))

	ciphertext, err := EncryptToMemo(1, alice, bob.Address(), plaintext)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("ciphertext", ciphertext, string(ciphertext[:]))
	if bytes.Equal(plaintext[:], ciphertext[:]) {
		t.Error("Cipher text and plaintext are the same")
	}

	deciphertext, err := EncryptToMemo(1, bob, alice.Address(), *ciphertext)
	if err != nil {
		t.Error(err)
	}
	fmt.Println("deciphertext", deciphertext, string(deciphertext[:]))

	if !bytes.Equal(plaintext[:], deciphertext[:]) {
		t.Errorf("plaintextes does not match plain: %v cipher: %v decipher: %v", plaintext[:], ciphertext[:], deciphertext[:])
	}
}
