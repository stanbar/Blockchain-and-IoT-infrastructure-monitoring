package main

import (
	"context"
	"fmt"
	"log"
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
	masterAcc := helpers.MustLoadMasterAccount()
	timeKeypairs := helpers.TimeIndexAccounts
	sensorKeypairs := helpers.DevicesKeypairs

	helpers.MustCreateAccounts(masterAcc, timeKeypairs)
	helpers.MustCreateAccounts(masterAcc, []*keypair.Full{functions.AssetKeypair})
	time.Sleep(5 * time.Second)

	timeAccounts := loadAccounts(masterAcc, timeKeypairs)
	sensorAccounts := loadAccounts(masterAcc, sensorKeypairs)
	assetAccount := helpers.MustLoadAccount(functions.AssetKeypair.Address())

	createTrustlines(masterAcc, timeKeypairs, timeAccounts)
	createTrustlines(masterAcc, sensorKeypairs, sensorAccounts)

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

	row := dbpool.QueryRow(context.Background(), "SELECT ledger_sequence FROM history_transactions WHERE account = $1 ORDER BY ledger_sequence ASC", sensorKeypairs[0].Address())
	var firstLedgerSeq int64
	err = row.Scan(&firstLedgerSeq)
	if err != nil {
		log.Fatal("Did not find first row")
	}

	row = dbpool.QueryRow(context.Background(), "SELECT ledger_sequence FROM history_transactions WHERE account = $1 ORDER BY ledger_sequence DESC", sensorKeypairs[0].Address())
	var lastLedgerSeq int64
	row.Scan(&lastLedgerSeq)
	if err != nil {
		log.Fatal("Did not find last row")
	}

	currentLedger := firstLedgerSeq
	for {
		avg, min, max := aggregator.CalculateFunctionsForLedger(dbpool, sensorKeypairs[0].Address(), currentLedger)
		log.Printf("avg: %d min: %d max: %d\n", avg, min, max)

		aggregator.SendAvgTransaction(timeAccounts[0], timeKeypairs[0], avg, sensorKeypairs[0].Address(), firstLedgerSeq, firstLedgerSeq+1)
		aggregator.SendMinTransaction(timeAccounts[0], timeKeypairs[0], min, sensorKeypairs[0].Address(), firstLedgerSeq, firstLedgerSeq+1)
		aggregator.SendMaxTransaction(timeAccounts[0], timeKeypairs[0], max, sensorKeypairs[0].Address(), firstLedgerSeq, firstLedgerSeq+1)
		if currentLedger == lastLedgerSeq {
			break
		}
	}
	log.Printf("Finished aggregating from %d to %d\n", firstLedgerSeq, lastLedgerSeq)
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
		accounts[i] = result.Account
	}
	return accounts
}

func createTrustlines(masterAcc *horizon.Account, keypairs []*keypair.Full, accounts []*horizon.Account) {
	timeIndexes := make([]helpers.CreateTrustline, len(keypairs))
	for i, v := range keypairs {
		timeIndexes[i] = TimeIndex{
			keypair: v,
			account: accounts[i],
		}
	}

	helpers.TryCreateTrustlines(masterAcc, timeIndexes, functions.Assets)
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
