const { Keypair, TransactionBuilder, Account, Asset, Operation, Transaction } = require("stellar-sdk");
const fetch = require('node-fetch');
const {submittingMessage, SendLogTx, defaultOptions } = require("./common")
const { parentPort } = require("worker_threads");


// Main thread will pass the data you need
// through this event listener.
parentPort.on("message", async (param) => {
  try{
    await sendLogTx(param)
    parentPort.postMessage({ok: true, error: undefined});
  } catch (e){
    parentPort.postMessage({ok: false, error: e});
  }
  // return the result to main thread.
});

async function sendLogTx({
  deviceId,
  index,
  server,
  batchAddress,
  deviceSecret,
  accountId,
  accountSeq,
}) {
  const deviceKeypair = Keypair.fromSecret(deviceSecret)
  const account = new Account(accountId, accountSeq)
  const tx = new TransactionBuilder(account, defaultOptions())
    // Create distribution account
    /* .addMemo(Memo.text(`${index}${deviceId}`)) */
    .addOperation(
      Operation.payment({
        destination: batchAddress,
        asset: Asset.native(),
        amount: `${1 / 10 ** 7}`,
      })
    )
    .setTimeout(0)
    .build();
  tx.sign(deviceKeypair);
  submittingMessage(index, deviceId)
  return sendTxToStellarCore(tx, server);
}

async function sendTxToStellarCore(tx, host) {
  const queryParams = new URLSearchParams({ blob: tx.toXDR() });
  const url = `${host}/tx?${queryParams}`;
  return fetch(url)
}

