package main

import (
	"log"
	"math/rand"
	"net/http"
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
		Timebounds:           txnbuild.NewTimeout(20),
		BaseFee:              100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return nil, err
	}
	signedTx, err := tx.Sign(networkPassphrase, signer)
	if err != nil {
		return nil, err
	}
	log.Println("Submitting createAccount transaction")
	response, err := client.SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	return &response, err
}

func sendLogTx(
	deviceId int,
	index int,
	server string,
	batchAddress *keypair.Full,
	iotDeviceKeypair *keypair.Full,
	account *horizon.Account,
) (*http.Response, error) {

	txParams := txnbuild.TransactionParams{
		SourceAccount:        account,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{&txnbuild.Payment{
			Destination: batchKeypair.Address(),
			Amount:      "10",
		}},
		Timebounds: txnbuild.NewTimeout(20),
		BaseFee:    100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		return nil, err
	}
	signedTx, err := tx.Sign(networkPassphrase, iotDeviceKeypair)
	if err != nil {
		return nil, err
	}
	xdr, err := signedTx.Base64()
	if err != nil {
		return nil, err
	}
	log.Println("Submitting sendLogTx transaction")
	req, err := http.NewRequest("GET", server, nil)
	if err != nil {
		log.Println(err)
	}
	q := req.URL.Query()
	q.Add("blob", xdr)
	req.URL.RawQuery = q.Encode()
	log.Println(req.URL.String())
	response, err := http.Get(req.URL.String())
	if err != nil {
		log.Println(err)
	}
	log.Println(response)
	return response, err
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

func chunkSlice(slice []*keypair.Full, chunkSize int) [][]*keypair.Full {
	var chunks [][]*keypair.Full
	for i := 0; i < len(slice); i += chunkSize {
		end := i + chunkSize
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}

type SendLog struct {
	deviceId int
	index int
	server string
	batchAddress *keypair.Full
	iotDeviceKeypair *keypair.Full
	account *horizon.Account
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
	res, err := createAccounts([]*keypair.Full{batchKeypair}, masterKp, masterAccount, randomServer())
	if err != nil {
		hError := err.(*horizonclient.Error)
		log.Println("Error submitting transaction:", hError.Problem.Extras["result_codes"])
	}
	if res != nil {
		log.Println(res.Successful)
	}

	chunks := chunkSlice(keypairs, 100)
	for _, chunk := range chunks {
		_, err := createAccounts(chunk, masterKp, masterAccount, randomServer())
		if err != nil {
			hError := err.(*horizonclient.Error)
			log.Fatal("Error submitting transaction:", hError.Problem.Extras)
		}
	}

	sendLogs := make([]*SendLog, len(keypairs))
	for i := 0; i < len(keypairs); i++ {
		sendLogs[i] = SendLog{
			deviceId : i,
			server: randomStellarCoreUrl(),
			batchAddress: batchKeypair,
			iotDeviceKeypair: keypairs[i],
			account: *horizon.Account
		}
		keypairs[i] = keypair.MustRandom()
	}

	iotAccounts := keypairs.map(async (kp, index) => ({
			deviceId: index,
			server: randomStellarCoreUrl(),
			keypair: kp,
			account: await randomServer().loadAccount(kp.publicKey()),
		}))

}
