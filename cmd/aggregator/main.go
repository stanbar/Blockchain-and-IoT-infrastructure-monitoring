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
	"github.com/stellot/stellot-iot/pkg/utils"
)

var batchKeypair = keypair.MustParseFull(utils.MustGetenv("BATCH_SECRET_KEY"))

func main() {

	dbpool, err := pgxpool.Connect(context.Background(), "postgres://stellar:jBH7qeurzt1wOCQ2@node1/core")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer dbpool.Close()

	rows, err := dbpool.Query(context.Background(), "SELECT txid, txbody FROM txhistory")
	for rows.Next() {
		var txid string
		var txbody string
		err := rows.Scan(&txid, &txbody)
		if err != nil {
			log.Fatal(err)
		}
		// fmt.Printf("%s, %s\n", txid, txbody)
		transaction, err := txnbuild.TransactionFromXDR(txbody)
		if err != nil {
			log.Fatal(err)
		}
		tx, ok := transaction.Transaction()
		if !ok {
			log.Fatal("Can not get simple transaction")
		}
		srcAccount := tx.SourceAccount()
		log.Println(srcAccount)
		genericMemo, ok := tx.Memo().(txnbuild.MemoHash)
		if !ok {
			log.Println("Can not cast memo to MemoHash")
			continue
		}
		memo := txnbuild.MemoHash(genericMemo)
		fmt.Println(string(memo[:]))
		seqNumber, err := srcAccount.GetSequenceNumber()
		if err != nil {
			log.Fatal(err)
		}
		decrypted, err := crypto.EncryptToMemo(seqNumber, batchKeypair, srcAccount.GetAccountID(), memo)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("memo:", string(decrypted[:]))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}
}
