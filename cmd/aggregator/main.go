package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/protocols/horizon"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/aggregator"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/functions"
	"github.com/stellot/stellot-iot/pkg/helpers"
)

func main() {
	http.DefaultClient.Timeout = time.Second * time.Duration(helpers.TimeOut)
	rand.Seed(time.Now().UnixNano())
	helpers.BlockUntilHorizonIsReady()
	masterAcc := helpers.MustLoadMasterAccount()
	timeKeypairs := helpers.TimeIndexAccounts
	sensorKeypairs := helpers.DevicesKeypairs

	helpers.MustCreateAccounts(masterAcc, timeKeypairs, "core")
	helpers.MustCreateAccounts(masterAcc, []*keypair.Full{functions.AssetKeypair}, "horizon")

	log.Println("Loading time accounts")
	timeAccounts := loadAccounts(masterAcc, timeKeypairs)

	log.Println("Loading sensor accounts")
	sensorAccounts := loadAccounts(masterAcc, sensorKeypairs)

	log.Println("Loading functions asset account")
	assetAccount := helpers.MustLoadAccount(functions.AssetKeypair.Address())

	createTrustlines(masterAcc, timeKeypairs, timeAccounts, "core")
	createTrustlines(masterAcc, sensorKeypairs, sensorAccounts, "core")

	for _, v := range functions.Assets {
		helpers.MustFundAccountsEvenly(masterAcc, assetAccount, functions.AssetKeypair, timeKeypairs, v)
	}

	// log.Println("Loaded tmie accounts", accounts)

	dbpool, err := pgxpool.Connect(context.Background(), helpers.DatabaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer dbpool.Close()

	aggregateForNBlocksInterval(dbpool, 1, sensorKeypairs[0], timeAccounts[0], timeKeypairs[0])
	aggregateForNBlocksInterval(dbpool, 2, sensorKeypairs[0], timeAccounts[1], timeKeypairs[1])
	aggregateForNBlocksInterval(dbpool, 3, sensorKeypairs[0], timeAccounts[2], timeKeypairs[2])
	aggregateForNBlocksInterval(dbpool, 6, sensorKeypairs[0], timeAccounts[3], timeKeypairs[3])
	aggregateForNBlocksInterval(dbpool, 12, sensorKeypairs[0], timeAccounts[4], timeKeypairs[4])
}

type TimeIndex struct {
	keypair *keypair.Full
	account *horizon.Account
}

func (t TimeIndex) Keypair() *keypair.Full {
	return t.keypair
}

func (t TimeIndex) Account() *horizon.Account {
	return t.account
}

func loadAccounts(masterAcc *horizon.Account, keypairs []*keypair.Full) []*horizon.Account {
	channels := make([]chan helpers.LoadAccountResult, len(keypairs))
	for i := 0; i < len(channels); i++ {
		channels[i] = helpers.LoadAccountChan(keypairs[i].Address())
	}
	accounts := make([]*horizon.Account, len(keypairs))
	for i := 0; i < len(keypairs); i++ {
		result := <-channels[i]
		if result.Error != nil {
			log.Fatal(result.Error)
		}
		accounts[i] = result.Account
	}
	return accounts
}

func createTrustlines(masterAcc *horizon.Account, keypairs []*keypair.Full, accounts []*horizon.Account, where string) {
	timeIndexes := make([]helpers.CreateTrustline, len(keypairs))
	for i, v := range keypairs {
		timeIndexes[i] = TimeIndex{
			keypair: v,
			account: accounts[i],
		}
	}

	helpers.MustCreateTrustlines(masterAcc, timeIndexes, functions.Assets, where)
}

func proceed(tx *txnbuild.Transaction, op *txnbuild.Payment) {
	srcAccount := tx.SourceAccount()
	genericMemo, ok := tx.Memo().(txnbuild.MemoHash)
	if !ok {
		log.Println("Can not cast memo to MemoHash")
		return
	}
	memo := txnbuild.MemoHash(genericMemo)
	seqNumber, err := srcAccount.GetSequenceNumber()
	if err != nil {
		log.Fatal(err)
	}
	decrypted, err := crypto.EncryptToMemo(seqNumber, helpers.BatchKeypair, srcAccount.GetAccountID(), memo)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("decrypted memo:", string(decrypted[:]))
}

func aggregateForNBlocksInterval(dbpool *pgxpool.Pool, blocks int64, aggregatingOn *keypair.Full, timeAccount *horizon.Account, timeKeypair *keypair.Full) {
	log.Printf("Aggregating for account = %s for and collecting on %s", aggregatingOn.Address(), timeAccount.AccountID)

	row := dbpool.QueryRow(context.Background(), "SELECT ledger_sequence FROM history_transactions WHERE account = $1 ORDER BY ledger_sequence ASC", aggregatingOn.Address())
	var firstLedgerSeq int64
	err := row.Scan(&firstLedgerSeq)
	if err != nil {
		log.Fatal("Did not find first row")
	}

	row = dbpool.QueryRow(context.Background(), "SELECT ledger_sequence FROM history_transactions WHERE account = $1 ORDER BY ledger_sequence DESC", aggregatingOn.Address())
	var lastLedgerSeq int64
	row.Scan(&lastLedgerSeq)
	if err != nil {
		log.Fatal("Did not find last row")
	}

	for currentLedger := firstLedgerSeq; currentLedger < lastLedgerSeq; currentLedger += blocks {
		avg, min, max, err := aggregator.CalculateFunctionsForLedgers(dbpool, aggregatingOn.Address(), currentLedger, currentLedger+blocks) // what ledgerSeq to use ?
		if err != nil {
			log.Println(err)
			continue
		}
		log.Printf("avg: %d min: %d max: %d\n", avg, min, max)

		aggregator.SendAvgTransaction(timeAccount, timeKeypair, avg, aggregatingOn.Address(), currentLedger, currentLedger+blocks)
		aggregator.SendMinTransaction(timeAccount, timeKeypair, min, aggregatingOn.Address(), currentLedger, currentLedger+blocks)
		aggregator.SendMaxTransaction(timeAccount, timeKeypair, max, aggregatingOn.Address(), currentLedger, currentLedger+blocks)
	}
	log.Printf("Finished aggregating 5 blocks from %d to %d\n", firstLedgerSeq, lastLedgerSeq)
}
