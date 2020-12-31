package main

import (
	"log"
	"math/rand"
	"os"
	"strings"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/generator-go/utils"
)

var networkPassphrase = utils.MustGetenv("NETWORK_PASSPHRASE")
var horizonServerUrls = utils.MustGetenv("HORIZON_SERVER_URLS")
var urls = strings.Split(horizonServerUrls, " ")
var client = horizonclient.Client{HorizonURL: urls[rand.Intn(len(urls))]}

func createAccounts(kp *keypair.Full, signer *keypair.Full, sourceAcc *horizon.Account, client horizonclient.Client) {
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
	pair, err := keypair.Random()
	if err != nil {
		log.Fatal(err)
	}

	masterKp, err := keypair.FromRawSeed(network.ID(networkPassphrase))
	accReq := horizonclient.AccountRequest{AccountID: pair.Address()}
	masterAccount, err := client.AccountDetail(accReq)
	if err != nil {
		log.Fatal(err)
	}
	createAccounts(pair, masterKp, &masterAccount, client)
}
