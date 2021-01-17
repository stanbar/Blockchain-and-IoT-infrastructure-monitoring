package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stellar/go/keypair"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/crypto"
	"github.com/stellot/stellot-iot/pkg/helpers"
	"github.com/stellot/stellot-iot/pkg/usecases"
	"github.com/stellot/stellot-iot/pkg/utils"
)

var batchKeypair = keypair.MustParseFull(utils.MustGetenv("BATCH_SECRET_KEY"))
var databaseUrl = utils.MustGetenv("DATABASE_URL")

func main() {

	dbpool, err := pgxpool.Connect(context.Background(), databaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer dbpool.Close()

	rows, err := dbpool.Query(context.Background(), "SELECT txid, txbody, ledgerseq, txindex, txresult, txmeta FROM txhistory LIMIT 500")
	for rows.Next() {
		var txid string
		var txbody string
		var ledgerseq int
		var txindex int
		var txresult string
		var txmeta string
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
	decrypted, err := crypto.EncryptToMemo(seqNumber, batchKeypair, srcAccount.GetAccountID(), memo)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("decrypted memo:", string(decrypted[:]))
}
