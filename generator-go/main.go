package main

import (
	"log"
	"math/rand"
	"os"
	"strings"

	hClient "github.com/stellar/go/clients/horizonclient"
	keypair "github.com/stellar/go/keypair"
	network "github.com/stellar/go/network"
	hProtocol "github.com/stellar/go/protocols/horizon"
	txnbuild "github.com/stellar/go/txnbuild"
)

func createAccounts(kp *keypair.Full, signer *keypair.Full, sourceAcc *hProtocol.Account, client hClient.Client) {
	networkPassphrase, present := os.LookupEnv("NETWORK_PASSPHRASE")
	if present != true {
		log.Panicln("NETWORK_PASSPHRASE must be set")
	}
	createAccountOp := txnbuild.CreateAccount{
		Destination: kp.Address(),
		Amount:      "10",
	}
	txParams := txnbuild.TransactionParams{
		SourceAccount:        sourceAcc,
		IncrementSequenceNum: true,
		Operations:           []txnbuild.Operation{&createAccountOp},
		Timebounds:           txnbuild.NewTimeout(0),
		BaseFee:              1,
	}
	tx, _ := txnbuild.NewTransaction(txParams)

	signedTx, _ := tx.Sign(networkPassphrase, signer)
	client.SubmitTransaction(signedTx)
}

func main() {
	networkPassphrase, present := os.LookupEnv("HORIZON_SERVER_URLS")
	if present != true {
		log.Panicln("HORIZON_SERVER_URLS must be set")
	}

	horizonServerUrls, present := os.LookupEnv("HORIZON_SERVER_URLS")
	if present != true {
		log.Panicln("HORIZON_SERVER_URLS must be set")
	}

	urls := strings.Split(horizonServerUrls, " ")
	client := hClient.Client{HorizonURL: urls[rand.Intn(len(urls))]}
	pair, err := keypair.Random()
	if err != nil {
		log.Fatal(err)
	}

	masterKp, err := keypair.FromRawSeed(network.ID(networkPassphrase))
	accountRequest := hClient.AccountRequest{AccountID: pair.Address()}
	masterAccount, err := client.AccountDetail(accountRequest)
	if err != nil {
		log.Fatal(err)
	}
	createAccounts(pair, masterKp, &masterAccount, client)

}
