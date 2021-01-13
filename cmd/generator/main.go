package main

import (
	"context"
	"errors"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

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
var peroid, _ = strconv.Atoi(utils.MustGetenv("PEROID"))
var sendTxTo = utils.MustGetenv("SEND_TX_TO")
var tps, _ = strconv.Atoi(utils.MustGetenv("TPS"))
var timeOut, _ = strconv.ParseInt(utils.MustGetenv("SEND_TO_CORE_TIMEOUT_SECONDS"), 10, 64)
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
	rateLimiter   *rate.Limiter
}

type SendLogResult struct {
	HTTPResponseBody string
	HorizonResponse  *horizon.Transaction
	Error            error
}

func main() {
	http.DefaultClient.Timeout = time.Second * time.Duration(timeOut)
	rand.Seed(time.Now().UnixNano())
	keypairs := helpers.DevicesKeypairs()
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
				log.Println("Error submitting transaction:", hError.Problem.Extras)
			}
		}
		if res != nil {
			log.Println(res.Successful)
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
			rateLimiter:   rate.NewLimiter(rate.Every(time.Duration(1000.0/tps)*time.Millisecond), 1),
		}
	}

	var wg sync.WaitGroup
	for _, iotDevice := range iotDevices {
		wg.Add(1)
		go func(params IotDevice, wg *sync.WaitGroup) {
			defer wg.Done()
			time.Sleep(time.Duration(1000.0*params.deviceId/len(iotDevices)) * time.Millisecond)
			for i := 0; i < logsNumber; i++ {
				ctx := context.Background()
				err := params.rateLimiter.Wait(ctx)
				if err != nil {
					log.Println("Error returned by limiter", err)
					return
				}
				sendLogTx(params, i)
			}
		}(iotDevice, &wg)
	}

	log.Println("Main: Waiting for workers to finish")
	wg.Wait()
	log.Println("Main: Completed")
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

func sendLogTx(params IotDevice, eventIndex int) SendLogResult {
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
		Timebounds: txnbuild.NewTimeout(timeOut),
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
		if err != nil {
			hError := err.(*horizonclient.Error)
			if hError.Problem.Extras != nil {
				if hError.Problem.Extras["result_codes"] != nil {
					log.Fatalf("Error submitting sendLogTx to horizon, log device: %d log no. %d error: %v", params.deviceId, eventIndex, hError.Problem.Extras["result_codes"])
				} else {
					log.Fatalf("Error submitting sendLogTx to horizon, log device: %d log no. %d error: %v", params.deviceId, eventIndex, hError.Problem.Extras)
				}
			} else {
				log.Fatalf("Error submitting sendLogTx to horizon, log device: %d log no. %d error: %s", params.deviceId, eventIndex, err)
			}
		}
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
			log.Printf("Error reading body of log device: %d log no. %d %v", params.deviceId, eventIndex, err)
		} else {
			log.Printf("Success sending log deviceId %d log no. %d %s", params.deviceId, eventIndex, string(body))
			if strings.Contains(string(body), "ERROR") {
				log.Fatalf("Received ERROR transactioin in deviceId %d log no. %d", params.deviceId, eventIndex)
			}
		}
		return SendLogResult{HTTPResponseBody: string(body), Error: err}
	} else {
		return SendLogResult{Error: errors.New("Unsupported sendTxTo")}
	}
}

func sendTxToHorizon(horizon *horizonclient.Client, xdr string) (horizon.Transaction, error) {
	return horizon.SubmitTransactionXDR(xdr)
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
