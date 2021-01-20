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
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/functions"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
)

func main() {
	masterAcc := helpers.MustLoadMasterAccount()
	accounts := createTimeIndexAccounts(masterAcc)
	log.Println("Loaded tmie accounts", accounts)

	dbpool, err := pgxpool.Connect(context.Background(), helpers.DatabaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer dbpool.Close()

	rows, err := dbpool.Query(context.Background(), "SELECT txid, txbody, txhistory.ledgerseq, txindex, txresult, txmeta, closetime FROM txhistory, ledgerheaders WHERE txhistory.ledgerseq = ledgerheaders.ledgerseq LIMIT 500")
	for rows.Next() {
		var (
			txid      string
			txbody    string
			ledgerseq int
			txindex   int
			txresult  string
			txmeta    string
		)
		err := rows.Scan(&txid, &txbody, &ledgerseq, &txindex, &txresult, &txmeta)
		if err != nil {
			log.Fatal(err)
		}
		transaction, err := txnbuild.TransactionFromXDR(txbody)
		if err != nil {
			log.Fatal(err)
		}
		tx, ok := transaction.Transaction()
		if !ok {
			log.Fatal("Can not get simple transaction")
		}
		ops := tx.Operations()
		for _, op := range ops {
			val, ok := op.(*txnbuild.Payment)
			if !ok {
				log.Println("Tx is not payment")
			} else {
				if val.Destination == helpers.BatchKeypair.Address() {
					log.Println("Sent to batch address")
					if val.Asset == usecases.TEMP.Asset() || val.Asset == usecases.HUMD.Asset() {
						log.Printf("Sent %s TEMP or HUMD\n", val.Amount)
						log.Printf("txid %s, ledgerseq %d, txindex %d\n", txid, ledgerseq, txindex)
						proceed(tx, val)
					}
				}
			}
		}
	}
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
