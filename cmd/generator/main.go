package main

import (
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
	"github.com/stellot/stellot-iot/pkg/utils"
)

var networkPassphrase = utils.MustGetenv("NETWORK_PASSPHRASE")
var logsNumber, _ = strconv.Atoi(utils.MustGetenv("LOGS_NUMBER"))
var noDevices, _ = strconv.Atoi(utils.MustGetenv("NO_DEVICES"))
var peroid, _ = strconv.Atoi(utils.MustGetenv("PEROID"))
var sendTxTo = utils.MustGetenv("SEND_TX_TO")
var tps, _ = strconv.Atoi(utils.MustGetenv("TPS"))
var timeOut, _ = strconv.Atoi(utils.MustGetenv("SEND_TO_CORE_TIMEOUT_SECONDS"))
var batchKeypair = keypair.MustParseFull(utils.MustGetenv("BATCH_SECRET_KEY"))
var masterKp, _ = keypair.FromRawSeed(network.ID(networkPassphrase))

type IotDevice struct {
	deviceId      int
	logValue      [32]byte
	index         int
	server        string
	horizon       *horizonclient.Client
	batchAddress  string
	deviceKeypair *keypair.Full
	account       *horizon.Account
}

type SendLogResult struct {
	HTTPResponseBody string
	HorizonResponse  *horizon.Transaction
	Error            error
}

func main() {
	http.DefaultClient.Timeout = time.Second * time.Duration(timeOut)
	rand.Seed(time.Now().UnixNano())
	keypairs := make([]*keypair.Full, noDevices)
	for i := 0; i < noDevices; i++ {
		keypairs[i] = keypair.MustRandom()
	}
	masterAccount, err := helpers.LoadMasterAccount()
	if err != nil {
		log.Fatal(err)
	}
	res, err := createAccounts([]*keypair.Full{batchKeypair}, masterKp, masterAccount, helpers.RandomHorizon())
	if err != nil {
		hError := err.(*horizonclient.Error)
		log.Println("Error submitting transaction:", hError.Problem.Extras["result_codes"])
	}
	if res != nil {
		log.Println(res.Successful)
	}

	chunks := utils.ChunkKeypairs(keypairs, 100)
	for _, chunk := range chunks {
		_, err := createAccounts(chunk, masterKp, masterAccount, helpers.RandomHorizon())
		if err != nil {
			hError, ok := err.(*horizonclient.Error)
			if ok {
				log.Fatal("Error submitting transaction:", hError.Problem.Extras)
			}
			log.Fatalf("Error submitting transaction %+v", err)
		}
	}

	channels := make([]chan LoadAccountResult, len(keypairs))
	for i := 0; i < len(channels); i++ {
		channels[i] = loadAccountChan(keypairs[i].Address())
	}

	iotDevices := make([]IotDevice, len(keypairs))
	for i := 0; i < len(keypairs); i++ {
		result := <-channels[i]
		if result.Error != nil {
			log.Println(result.Error)
			hError := result.Error.(*horizonclient.Error)
			log.Println("Error submitting transaction:", hError.Problem.Extras["result_codes"])
		}
		iotDevices[i] = IotDevice{
			deviceId:      i,
			server:        helpers.RandomStellarCoreUrl(),
			horizon:       helpers.RandomHorizon(),
			batchAddress:  batchKeypair.Address(),
			deviceKeypair: keypairs[i],
			account:       result.Account,
		}
	}

	resultChan := make(chan SendLogResult)
	for _, iotDevice := range iotDevices {
		go func(params IotDevice, resultChan chan SendLogResult) {
			time.Sleep(time.Duration(10.0*params.deviceId) * time.Millisecond)
			for i := 0; i < logsNumber; i++ {
				log.Printf("device %d goes to sleep %s", params.deviceId, time.Duration(1000.0/tps)*time.Millisecond)
				time.Sleep(time.Duration(1000.0/tps) * time.Millisecond)
				resultChan <- sendLogTx(params)
			}
		}(iotDevice, resultChan)
	}

	for i := 0; i < logsNumber*len(keypairs); i++ {
		result := <-resultChan
		if result.Error != nil {
			log.Printf("Error sending log %d %+v\n", i, result.Error)
		}
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
		Timebounds:           txnbuild.NewTimeout(120),
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

func sendLogTx(params IotDevice) SendLogResult {
	seqNum, err := strconv.ParseInt(params.account.Sequence, 10, 64)
	if err != nil {
		return SendLogResult{Error: err}
	}

	logValue := usecases.RandomTemperature(params.index + params.deviceId)
	payload, err := crypto.EncryptToMemo(seqNum+1, params.deviceKeypair, params.batchAddress, logValue)
	memo := txnbuild.MemoHash(*payload)

	txParams := txnbuild.TransactionParams{
		SourceAccount:        params.account,
		IncrementSequenceNum: true,
		Operations: []txnbuild.Operation{&txnbuild.Payment{
			Destination: params.batchAddress,
			Asset:       txnbuild.NativeAsset{},
			Amount:      "0.0000001",
		}},
		Memo:       memo,
		Timebounds: txnbuild.NewTimeout(20),
		BaseFee:    100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		log.Println("Error creating new transaction", err)
		return SendLogResult{Error: err}
	}
	signedTx, err := tx.Sign(networkPassphrase, params.deviceKeypair)
	if err != nil {
		log.Println("Error signing transaction", err)
		return SendLogResult{Error: err}
	}
	xdr, err := signedTx.Base64()
	if err != nil {
		log.Println("Error converting to base64", err)
		return SendLogResult{Error: err}
	}

	if sendTxTo == "horizon" {
		resp, err := sendTxToHorizon(params.horizon, xdr)
		return SendLogResult{HorizonResponse: &resp, Error: err}
	} else if sendTxTo == "stellar-core" {
		response, err := sendTxToStellarCore(params.server, xdr)
		if err != nil {
			uError := err.(*url.Error)
			log.Printf("Error sending get request to stellar core %+v\n", uError)
		}
		defer response.Body.Close()
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Printf("Error reading body of log %d %v", params.deviceId, err)
		} else {
			log.Printf("Success sending log %d %s", params.deviceId, string(body))
		}
		return SendLogResult{HTTPResponseBody: string(body), Error: err}
	} else {
		return SendLogResult{Error: errors.New("Unsupported sendTxTo")}
	}
}

func sendTxToHorizon(horizon *horizonclient.Client, xdr string) (resp horizon.Transaction, err error) {
	resp, err = horizon.SubmitTransactionXDR(xdr)

	if err != nil {
		hError := err.(*horizonclient.Error)
		if hError.Problem.Extras != nil {
			if hError.Problem.Extras["result_codes"] != nil {
				log.Println("Error submitting sendLogTx to horizon", hError.Problem.Extras["result_codes"])
			} else {
				log.Println("Error submitting sendLogTx to horizon", hError.Problem.Extras)
			}
		}
	}
	return
}

func sendTxToStellarCore(server string, xdr string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", server+"/tx", nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	q.Add("blob", xdr)
	req.URL.RawQuery = q.Encode()
	return http.Get(req.URL.String())
}

type LoadAccountResult struct {
	Account *horizon.Account
	Error   error
}

func loadAccountChan(accountId string) chan LoadAccountResult {
	ch := make(chan LoadAccountResult)
	accReq := horizonclient.AccountRequest{AccountID: accountId}
	go func() {
		masterAccount, err := helpers.RandomHorizon().AccountDetail(accReq)
		if err != nil {
			ch <- LoadAccountResult{Account: nil, Error: err}
		} else {
			ch <- LoadAccountResult{Account: &masterAccount, Error: nil}
		}
	}()
	return ch
}
