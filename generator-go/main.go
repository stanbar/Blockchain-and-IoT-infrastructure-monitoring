package main

import (
	"log"
	"math/rand"
	"strconv"
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
var noDevices, _ = strconv.Atoi(utils.MustGetenv("NO_DEVICES"))
var urls = strings.Split(horizonServerUrls, " ")
var masterKp, _ = keypair.FromRawSeed(network.ID(networkPassphrase))
var batchKeypair = keypair.MustParseFull(utils.MustGetenv("BATCH_SECRET_KEY"))

func randomServer() *horizonclient.Client {
	return &horizonclient.Client{
		HorizonURL: urls[rand.Intn(len(urls))],
	}
}

func createAccounts(kp []*keypair.Full, signer *keypair.Full, sourceAcc *horizon.Account, client *horizonclient.Client) (*horizon.Transaction, error) {
	createAccountOps := make([]txnbuild.Operation, len(kp))

	for i, v := range kp {
		createAccountOps[i] = &txnbuild.CreateAccount{
			Destination: v.Address(),
			Amount:      "10",
		}
	}
	txParams := txnbuild.TransactionParams{
		SourceAccount:        sourceAcc,
		IncrementSequenceNum: true,
		Operations:           createAccountOps,
		Timebounds:           txnbuild.NewTimeout(0),
		BaseFee:              1,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return nil, err
	}
	signedTx, err := tx.Sign(networkPassphrase, signer)
	if err != nil {
		return nil, err
	}
	response, err := client.SubmitTransaction(signedTx)
	return &response, err

}
func loadMasterAccount() (*horizon.Account, error) {
	log.Println("Loading master account")
	accReq := horizonclient.AccountRequest{AccountID: masterKp.Address()}
	masterAccount, err := randomServer().AccountDetail(accReq)
	if err != nil {
		return nil, err
	}
	log.Println("Loaded master account")
	return &masterAccount, nil
}

func main() {
	keypairs := make([]*keypair.Full, noDevices)
	for i := 0; i < noDevices; i++ {
		keypairs[i] = keypair.MustRandom()
	}
	masterAccount, err := loadMasterAccount()
	if err != nil {
		log.Fatal(err)
	}
	createAccounts([]*keypair.Full{batchKeypair}, masterKp, masterAccount, randomServer())
}
