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

var stellarCoreUrls = utils.MustGetenv("STELLAR_CORE_URLS")
var stellarCoreUrlsSlice = strings.Split(stellarCoreUrls, " ")
var networkPassphrase = utils.MustGetenv("NETWORK_PASSPHRASE")
var horizonServerUrls = utils.MustGetenv("HORIZON_SERVER_URLS")
var logsNumber, _ = strconv.Atoi(utils.MustGetenv("LOGS_NUMBER"))
var noDevices, _ = strconv.Atoi(utils.MustGetenv("NO_DEVICES"))
var urls = strings.Split(horizonServerUrls, " ")
var masterKp, _ = keypair.FromRawSeed(network.ID(networkPassphrase))
var batchKeypair = keypair.MustParseFull(utils.MustGetenv("BATCH_SECRET_KEY"))

func randomServer() *horizonclient.Client {
	return &horizonclient.Client{
		HorizonURL: urls[rand.Intn(len(urls))],
	}
}

func randomStellarCoreUrl() string {
	return stellarCoreUrlsSlice[rand.Intn(len(stellarCoreUrlsSlice))]
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

func loadAccount(accountId string) (*horizon.Account, error) {
	accReq := horizonclient.AccountRequest{AccountID: accountId}
	masterAccount, err := randomServer().AccountDetail(accReq)
	if err != nil {
		return nil, err
	}
	return &masterAccount, nil
}

type LoadAccountResult struct {
	Account *horizon.Account
	Error   error
}

func loadAccountChan(accountId string) chan LoadAccountResult {
	ch := make(chan LoadAccountResult)
	accReq := horizonclient.AccountRequest{AccountID: accountId}
	go func() {
		log.Println("Sending account details request")
		masterAccount, err := randomServer().AccountDetail(accReq)
		log.Println("Response received")
		if err != nil {
			ch <- LoadAccountResult{Account: nil, Error: err}
		} else {
			ch <- LoadAccountResult{Account: &masterAccount, Error: nil}
		}
	}()
	return ch
}

func loadMasterAccount() (*horizon.Account, error) {
	return loadAccount(masterKp.Address())
}

func chunkSlice(slice []*keypair.Full, chunkSize int) [][]*keypair.Full {
	var chunks [][]*keypair.Full
	for {
		if len(slice) == 0 {
			break
		}
		// necessary check to avoid slicing beyond
		// slice capacity
		if len(slice) < chunkSize {
			chunkSize = len(slice)
		}
		chunks = append(chunks, slice[0:chunkSize])
		slice = slice[chunkSize:]
	}
	return chunks
}

type SendLog struct {
	deviceId      int
	index         int
	server        string
	batchAddress  *keypair.Full
	deviceKeypair *keypair.Full
	account       *horizon.Account
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

	channels := make([]chan LoadAccountResult, len(keypairs))
	for i := 0; i < len(channels); i++ {
		channels[i] = loadAccountChan(keypairs[i].Address())
	}

	sendLogs := make([]SendLog, len(keypairs))
	for i := 0; i < len(keypairs); i++ {
		log.Printf("Waiting for channel return %d", i)
		result := <-channels[i]
		log.Printf("Response received %d", i)
		if result.Error != nil {
			log.Fatal(result.Error)
			hError := result.Error.(*horizonclient.Error)
			log.Println("Error submitting transaction:", hError.Problem.Extras["result_codes"])
		}
		sendLogs[i] = SendLog{
			deviceId:      i,
			server:        randomStellarCoreUrl(),
			batchAddress:  batchKeypair,
			deviceKeypair: keypairs[i],
			account:       result.Account,
		}
	}

	for i := 0; i < logsNumber; i++ {
		for j := 0; j < len(keypairs); j++ {
			params := sendLogs[i]
			log.Printf("Sending log tx %d%d", i, j)
			sendLogTx(params.deviceId, i, params.server, params.batchAddress, params.deviceKeypair, params.account)
		}
	}
}
