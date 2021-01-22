package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
	// masterAcc := helpers.MustLoadMasterAccount()
	// accounts := createTimeIndexAccounts(masterAcc)
	// log.Println("Loaded tmie accounts", accounts)
	sensors := helpers.DevicesKeypairs

	dbpool, err := pgxpool.Connect(context.Background(), helpers.DatabaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer dbpool.Close()

	row := dbpool.QueryRow(context.Background(), "SELECT ledger_sequence FROM history_transactions WHERE account = $1 ORDER BY ledger_sequence ASC", sensors[0].Address())
	var firstLedgerSeq int
	row.Scan(&firstLedgerSeq)

	row = dbpool.QueryRow(context.Background(), "SELECT ledger_sequence FROM history_transactions WHERE account = $1 ORDER BY ledger_sequence DESC", sensors[0].Address())
	var lastLedgerSeq int
	row.Scan(&lastLedgerSeq)

	avg, min, max := aggregator.CalculateFunctionsForLedger(dbpool, sensors[0].Address(), firstLedgerSeq)
	log.Printf("avg %d min %d max %d\n", avg, min, max)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	} else {
		log.Println("Done")
	}
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

func createTimeIndexAccounts(masterAcc *horizon.Account) []*horizon.Account {
	keypairs := []*keypair.Full{helpers.FiveSecondsKeypair, helpers.TenSecondsKeypair, helpers.ThirtySecondsKeypair, helpers.OneMinuteKeypair}
	helpers.MustCreateAccounts(masterAcc, keypairs)

	channels := make([]chan helpers.LoadAccountResult, len(keypairs))
	for i := 0; i < len(channels); i++ {
		channels[i] = helpers.LoadAccountChan(keypairs[i].Address())
	}
	accounts := make([]*horizon.Account, len(keypairs))
	for i := 0; i < len(keypairs); i++ {
		result := <-channels[i]
		accounts[i] = result.Account
	}

	devices := make([]helpers.CreateTrustline, len(keypairs))
	for i, v := range keypairs {
		devices[i] = TimeIndex{
			keypair: v,
			account: accounts[i],
		}
	}

	helpers.TryCreateTrustlines(masterAcc, devices, functions.Assets)
	return accounts
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
