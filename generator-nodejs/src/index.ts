import {
  Account,
  Asset,
  Memo,
  Config,
  Operation,
  Keypair,
  Server,
  Transaction,
  TransactionBuilder,
} from "stellar-sdk";
import { generate } from "./utils";
import { chunk } from "lodash";
import fetch from "node-fetch";
import { StaticPool } from "node-worker-threads-pool";

const {
  NETWORK_PASSPHRASE,
  HORIZON_SERVER_URLS,
  BATCH_SECRET_KEY,
  LOGS_NUMBER,
  PEROID,
  STELLAR_CORE_URLS,
  NO_DEVICES,
  TPS,
} = process.env;
if (!NETWORK_PASSPHRASE) {
  throw new Error("NETWORK_PASSPHRASE must be defined");
}
if (!HORIZON_SERVER_URLS) {
  throw new Error("HORIZON_SERVER_URLS must be defined");
}
if (!BATCH_SECRET_KEY) {
  throw new Error("BATCH_SECRET_KEY must be defined");
}
if (!NO_DEVICES) {
  throw new Error("NO_DEVICES must be defined");
}
if (!TPS) {
  throw new Error("TPS must be defined");
}
if (!LOGS_NUMBER) {
  throw new Error("LOGS_NUMBER must be defined");
}
if (!STELLAR_CORE_URLS) {
  throw new Error("STELLAR_CORE_URLS must be defined");
}
if (!PEROID) {
  throw new Error("PEROID must be defined");
}
console.log({
  NETWORK_PASSPHRASE,
  HORIZON_SERVER_URLS,
  BATCH_SECRET_KEY,
  LOGS_NUMBER,
  STELLAR_CORE_URLS,
  PEROID,
  NO_DEVICES,
  TPS,
});

Config.setAllowHttp(true);
const masterKeypair = Keypair.master(NETWORK_PASSPHRASE);
const horizonUrls: string[] = HORIZON_SERVER_URLS.split(" ");
const stellarCoreUrls: string[] = STELLAR_CORE_URLS.split(" ");
const servers = horizonUrls.map((url) => new Server(url, { allowHttp: true }));
const randomServer = () => servers[Math.floor(Math.random() * servers.length)];
const randomStellarCoreUrl = () =>
  stellarCoreUrls[Math.floor(Math.random() * stellarCoreUrls.length)];

const sendLogTxPool = new StaticPool({
  size: 4,
  task: (params: SendLogTx) => sendLogTx(params),
});

/* const server = new Server(horizonUrls, { allowHttp: true }); */
console.log({
  masterKeypair: {
    public: masterKeypair.publicKey(),
    secret: masterKeypair.secret(),
  },
});

async function defaultOptions(): Promise<
  TransactionBuilder.TransactionBuilderOptions
> {
  /* console.log("Loadiing timebounds"); */
  /* const timebounds = await server.fetchTimebounds(10); */
  /* console.log("Loaded timebounds"); */
  return {
    networkPassphrase: NETWORK_PASSPHRASE,
    fee: "1",
  };
}

async function createAccounts(newKeypair: Keypair[], masterAccount: Account) {
  const builder = new TransactionBuilder(masterAccount, await defaultOptions());
  newKeypair.forEach((kp) => {
    builder.addOperation(
      Operation.createAccount({
        destination: kp.publicKey(),
        startingBalance: "100",
      })
    );
  });
  // Create distribution account
  const tx = builder.setTimeout(0).build();
  tx.sign(masterKeypair);
  console.log("Submitting createAccount transaction");
  return randomServer().submitTransaction(tx);
}

type SendLogTx = {
  deviceId: number;
  index: number;
  server: string;
  batchAddress: string;
  deviceKeypair: Keypair;
  account: Account;
};

async function sendLogTx({
  deviceId,
  index,
  server,
  batchAddress,
  deviceKeypair,
  account,
}: SendLogTx) {
  const tx = new TransactionBuilder(account, await defaultOptions())
    // Create distribution account
    .addMemo(Memo.text(`${index}${deviceId}`))
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
  console.log(
    `[${format2Digit(index)}${format3Digit(
      deviceId
    )}] Submitting log transaction`
  );
  return sendTxToStellarCore(tx, server);
  /* return server.submitTransaction(tx); */
}

async function sendTxToStellarCore(tx: Transaction, host: string) {
  console.log({ xdr: tx.toXDR() });
  const queryParams = new URLSearchParams({ blob: tx.toXDR() });
  const url = `${host}/tx?${queryParams}`;
  console.log(url);
  const res = await fetch(url);
  return res.json();
}

const format3Digit = (text: any) => formatDigit(3)(text);
const format2Digit = (text: any) => formatDigit(2)(text);
const formatDigit = (digits: number) => (text: any) =>
  text.toLocaleString(undefined, {
    minimumIntegerDigits: digits,
    useGrouping: false,
  });

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
    const masterAccount = await randomServer().loadAccount(
      masterKeypair.publicKey()
    );
    console.log("Loaded master account");
    await idempotent(() => createAccounts([batchKeypair], masterAccount));
    for (const keypairsChunk of chunk(keypairs, 100)) {
      await idempotent(() => createAccounts(keypairsChunk, masterAccount));
    }

    await wait(1000); // close ledger

    const params: Omit<SendLogTx, "index">[] = await Promise.all(
      keypairs.map(async (kp, index) => ({
        deviceId: index,
        server: randomStellarCoreUrl(),
        batchAddress: batchKeypair.publicKey(),
        deviceKeypair: kp,
        account: await randomServer().loadAccount(kp.publicKey()),
      }))
    );

    while (sent < Number(LOGS_NUMBER!)) {
      console.log(`Waiting ${Number(PEROID!) / 1000}s`);
      console.log(`Sending batch ${sent}`);
      for (const param of params) {
        await wait(Number(TPS!)/1000);
        sendLogTxPool.exec({...param, index: sent})
      }
      sent += 1;
    }
  } catch (err) {
    handleError(err, sent);
  }
}

function handleError(err: any, sent: number) {
  console.error(`[${sent}] Error ${err.message}`);
  if (err?.response?.data) {
    console.error(err.response.data);
    console.error(err.response.data.extras?.result_codes);
  } else if (err?.response) {
    console.error(err.response);
  }
}

async function wait(period: number) {
  return new Promise((resolve) => {
    setTimeout(resolve, period);
  });
}

main().catch(console.error);
