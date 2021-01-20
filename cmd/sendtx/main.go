package main

import (
	"log"

	"github.com/stellar/go/clients/horizonclient"
	"github.com/stellar/go/txnbuild"
	"github.com/stellot/stellot-iot/pkg/helpers"
)

func main() {
	keypairs := helpers.DevicesKeypairs
	sourceAcc := helpers.MustLoadAccount(keypairs[0].Address())
	log.Println("Source", keypairs[0].Address())
	log.Println("Destination", keypairs[1].Address())

	ops := []txnbuild.Operation{&txnbuild.BumpSequence{
		BumpTo: 33965,
	}, &txnbuild.Payment{
		Asset:       txnbuild.NativeAsset{},
		Amount:      "1",
		Destination: keypairs[1].Address(),
	}, &txnbuild.BumpSequence{
		BumpTo: 33968,
	}}

	txParams := txnbuild.TransactionParams{
		SourceAccount:        sourceAcc,
		IncrementSequenceNum: true,
		Operations:           ops,
		Timebounds:           txnbuild.NewTimeout(120),
		BaseFee:              100,
	}

	tx, err := txnbuild.NewTransaction(txParams)
	if err != nil {
		log.Fatal(err)
	}

	signedTx, err := tx.Sign(helpers.NetworkPassphrase, keypairs[0])
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Submitting createReceiverTrustlines transaction")
	resp, err := helpers.RandomHorizon().SubmitTransactionWithOptions(signedTx, horizonclient.SubmitTxOpts{SkipMemoRequiredCheck: true})
	HandleGracefuly(err)
	log.Println(resp)
}

func HandleGracefuly(err error) {
	if err != nil {
		hError, ok := err.(*horizonclient.Error)
		if ok {
			if hError.Problem.Extras["result_codes"] != nil {
				log.Printf("Error submitting tx result_codes: %s\n", hError.Problem.Extras["result_codes"])
			} else if hError.Problem.Extras["envelope_xdr"] != nil {
				log.Printf("Error submitting tx envelope_xdr: %s\n", hError.Problem.Extras["envelope_xdr"])
			} else if hError != nil {
				log.Printf("Error submitting tx: %v %s\n", hError, hError.Problem)
			}
		} else {
			log.Printf("Error submitting tx: %s\n", err)
		}
	} else {
		log.Println("Successfully submitted tx")
	}
}
