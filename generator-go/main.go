package main

import (
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/network"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/generator-go/crypto"
	"github.com/stellot/stellot-iot/generator-go/helpers"
	"github.com/stellot/stellot-iot/generator-go/usecases"
	"github.com/stellot/stellot-iot/generator-go/utils"
)

var networkPassphrase = utils.MustGetenv("NETWORK_PASSPHRASE")
var logsNumber, _ = strconv.Atoi(utils.MustGetenv("LOGS_NUMBER"))
var noDevices, _ = strconv.Atoi(utils.MustGetenv("NO_DEVICES"))
var peroid, _ = strconv.Atoi(utils.MustGetenv("PEROID"))
var sendTxTo = utils.MustGetenv("SEND_TX_TO")
var tps, _ = strconv.Atoi(utils.MustGetenv("TPS"))
var batchKeypair = keypair.MustParseFull(utils.MustGetenv("BATCH_SECRET_KEY"))
var masterKp, _ = keypair.FromRawSeed(network.ID(networkPassphrase))

func main() {
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
			hError := err.(*horizonclient.Error)
			log.Fatal("Error submitting transaction:", hError.Problem.Extras)
		}
	}

	channels := make([]chan LoadAccountResult, len(keypairs))
	for i := 0; i < len(channels); i++ {
		channels[i] = loadAccountChan(keypairs[i].Address())
	}

	sendLogs := make([]SendLogParams, len(keypairs))
	for i := 0; i < len(keypairs); i++ {
		result := <-channels[i]
		if result.Error != nil {
			log.Println(result.Error)
			hError := result.Error.(*horizonclient.Error)
			log.Println("Error submitting transaction:", hError.Problem.Extras["result_codes"])
		}
		sendLogs[i] = SendLogParams{
			deviceId:      i,
			logValue:      usecases.RandomTemperature(i),
			server:        helpers.RandomStellarCoreUrl(),
			horizon:       helpers.RandomHorizon(),
			batchAddress:  batchKeypair.Address(),
			deviceKeypair: keypairs[i],
			account:       result.Account,
		}
	}

	resultChans := make([]chan SendLogResult, logsNumber*len(keypairs))
	for i := 0; i < logsNumber; i++ {
		for j := 0; j < len(keypairs); j++ {
			log.Println("sleep", time.Duration(1000.0/tps)*time.Millisecond)
			time.Sleep(time.Duration(1000.0/tps) * time.Millisecond)
			params := sendLogs[j]
			log.Printf("Sending log tx %d%d index: %d", i, j, i*len(keypairs)+j)
			resultChans[i*len(keypairs)+j] = sendLogTx(params)
		}
	}

	for i := 0; i < logsNumber*len(keypairs); i++ {
		result := <-resultChans[i]
		if result.Error != nil {
			log.Printf("Error sending log %d %+v\n", i, result.Error)
		} else if result.HTTPResponse != nil {
			defer result.HTTPResponse.Body.Close()
			body, err := ioutil.ReadAll(result.HTTPResponse.Body)
			if err != nil {
				log.Printf("Error reading body of log %d %v", i, err)
			} else {
				log.Printf("Success sending log %d %s", i, string(body))
			}
		} else if result.HorizonResponse != nil {
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

type SendLogParams struct {
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
	HTTPResponse    *http.Response
	HorizonResponse *horizon.Transaction
	Error           error
}

func sendLogTx(params SendLogParams) chan SendLogResult {
	ch := make(chan SendLogResult)
	go func() {

		seqNum, err := strconv.Atoi(params.account.Sequence)
		if err != nil {
			ch <- SendLogResult{Error: err}
			return
		}
		payload, err := crypto.EncryptToMemo(seqNum+1, params.deviceKeypair, params.batchAddress, params.logValue)
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
			ch <- SendLogResult{Error: err}
			return
		}
		signedTx, err := tx.Sign(networkPassphrase, params.deviceKeypair)
		if err != nil {
			ch <- SendLogResult{Error: err}
			return
		}
		xdr, err := signedTx.Base64()
		if err != nil {
			ch <- SendLogResult{Error: err}
			return
		}

		if sendTxTo == "horizon" {
			resp, err := sendTxToHorizon(params.horizon, xdr)
			ch <- SendLogResult{HorizonResponse: &resp, Error: err}
		} else if sendTxTo == "stellar-core" {
			response, err := sendTxToStellarCore(params.server, xdr)
			ch <- SendLogResult{HTTPResponse: response, Error: err}
		} else {
			ch <- SendLogResult{Error: errors.New("Unsupported sendTxTo")}
		}
	}()
	return ch
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
