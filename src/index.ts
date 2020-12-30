import {
  Account,
  Asset,
  Memo,
  Config,
  Operation,
  Keypair,
  Server,
  TransactionBuilder,
} from "stellar-sdk";
import { generate } from "./utils";

const {
  NETWORK_PASSPHRASE,
  HORIZON_SERVER_URL,
  BATCH_SECRET_KEY,
  LOGS_NUMBER,
  PEROID,
  NO_DEVICES,
  TOTAL_TPS,
} = process.env;
if (!NETWORK_PASSPHRASE) {
  throw new Error("NETWORK_PASSPHRASE must be defined");
}
if (!HORIZON_SERVER_URL) {
  throw new Error("HORIZON_SERVER_URL must be defined");
}
if (!BATCH_SECRET_KEY) {
  throw new Error("BATCH_SECRET_KEY must be defined");
}
if (!NO_DEVICES) {
  throw new Error("NO_DEVICES must be defined");
}
if (!LOGS_NUMBER && !TOTAL_TPS) {
  throw new Error("LOGS_NUMBER or TPS must be defined");
}
if (!PEROID) {
  throw new Error("PEROID must be defined");
}
console.log({
  NETWORK_PASSPHRASE,
  HORIZON_SERVER_URL,
  BATCH_SECRET_KEY,
  LOGS_NUMBER,
  PEROID,
  NO_DEVICES,
  TOTAL_TPS,
});

Config.setAllowHttp(true);
const masterKeypair = Keypair.master(NETWORK_PASSPHRASE);
const server = new Server(HORIZON_SERVER_URL, { allowHttp: true });
console.log({
  masterKeypair: {
    public: masterKeypair.publicKey(),
    secret: masterKeypair.secret(),
  },
});

async function defaultOptions(): Promise<TransactionBuilder.TransactionBuilderOptions> {
  console.log("Loadiing timebounds");
  const timebounds = await server.fetchTimebounds(10);
  console.log("Loaded timebounds");
  return {
    networkPassphrase: NETWORK_PASSPHRASE,
    fee: "1",
    timebounds,
  };
}

async function createAccount(newKeypair: Keypair, masterAccount: Account) {
  console.log(`masterAccount seqNumber: ${masterAccount.sequenceNumber()}`)
  const tx = new TransactionBuilder(masterAccount, await defaultOptions())
    // Create distribution account
    .addOperation(
      Operation.createAccount({
        destination: newKeypair.publicKey(),
        startingBalance: "100",
      })
    )
    .build();
  tx.sign(masterKeypair);
  console.log("Submitting createAccount transaction");
  return server.submitTransaction(tx);
}

async function sendLogTx(
  eventType: number,
  iotDeviceKeypair: Keypair,
  batchAddress: string
) {
  console.log("Loading iot device keypair");
  const deviceAccount = await server.loadAccount(iotDeviceKeypair.publicKey());
  console.log("Loaded iot device account");
  const tx = new TransactionBuilder(deviceAccount, await defaultOptions())
    // Create distribution account
    .addMemo(Memo.text(`${eventType}`))
    .addOperation(
      Operation.payment({
        destination: batchAddress,
        asset: Asset.native(),
        amount: `${1 / 10 ** 7}`,
      })
    )
    .build();
  tx.sign(iotDeviceKeypair);
  console.log("Submitting log transaction");
  return server.submitTransaction(tx);
}

const idempotent = async <T>(f: () => Promise<T>): Promise<T | void> => {
  try {
    return await f();
  } catch (err) {
    if (
      Array.isArray(err?.response?.data?.extras?.result_codes?.operations) &&
      err?.response?.data?.extras?.result_codes?.operations[0] ===
        "op_already_exists"
    ) {
      // Idempotent
    } else {
      throw err;
    }
  }
};

async function main() {
  const batchKeypair = Keypair.fromSecret(BATCH_SECRET_KEY!);
  console.log(`Batch publicKey ${batchKeypair.publicKey()}`);
  const keypairs = generate(Number(NO_DEVICES!))<Keypair>(() =>
    Keypair.random()
  );

  let sent = 0;
  try {
    console.log("Loading master account");
    const masterAccount = await server.loadAccount(masterKeypair.publicKey());
    console.log("Loaded master account");
    await idempotent(() => createAccount(batchKeypair, masterAccount));

    for await (const keypair of keypairs) {
      await idempotent(() => createAccount(keypair, masterAccount));
    }

    await wait(5000); // close ledger

    while (sent < Number(LOGS_NUMBER!)) {
      console.log(`Waiting ${Number(PEROID!) / 1000}s`);
      await wait(Number(PEROID!));
      console.log(`Sending batch ${sent}`);
      await Promise.all(
        keypairs.map((keypair) =>
          sendLogTx(sent, keypair, batchKeypair.publicKey())
        )
      );
      sent += 1;
    }
  } catch (err) {
    console.error(`[${sent}] Error ${err.message}`);
    if(err?.response?.data){
      console.error(err?.response?.data);
    }
    if(err?.response?.data){
      console.error(err?.response?.data?.extras?.result_codes);
    }
  }
}

async function wait(period: number) {
  return new Promise((resolve) => {
    setTimeout(resolve, period);
  });
}

main().catch(console.error);
