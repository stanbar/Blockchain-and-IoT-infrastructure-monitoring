import {
  Account,
  Config,
  Operation,
  Keypair,
  Server,
  TransactionBuilder,
} from "stellar-sdk";
import os from "os";
import { generate } from "./utils";
import { chunk } from "lodash";
import path from 'path';
import { SendLogTx, successMessage, defaultOptions } from "./common";
import WorkerPool from "./worker_pool";

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

const pool = new WorkerPool(os.cpus().length * 2);

/* const server = new Server(horizonUrls, { allowHttp: true }); */
console.log({
  masterKeypair: {
    public: masterKeypair.publicKey(),
    secret: masterKeypair.secret(),
  },
});

async function createAccounts(newKeypair: Keypair[], masterAccount: Account) {
  const builder = new TransactionBuilder(masterAccount, defaultOptions());
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

    const params = await Promise.all(
      keypairs.map(async (kp, index) => {
        const account = await randomServer().loadAccount(kp.publicKey());
        return {
          deviceId: index,
          server: randomStellarCoreUrl(),
          batchAddress: batchKeypair.publicKey(),
          deviceKeypair: kp,
          account,
        };
      })
    );

    const peroid = Number(TPS!) / 1000
    const logsNumber = Number(LOGS_NUMBER!)
    const start = process.hrtime();
    const totalTasks = logsNumber * params.length
    let finished = 0;
    while (sent < logsNumber) {
      console.log(`Waiting ${Number(PEROID!) / 1000}s`);
      console.log(`Sending batch ${sent}`);
      for (const param of params) {
        /* await wait(peroid); */
        const linearized: SendLogTx = {
          deviceId: param.deviceId,
          index: sent,
          server: param.server,
          batchAddress: param.batchAddress,
          deviceSecret: param.deviceKeypair.secret(),
          accountId: param.account.accountId(),
          accountSeq: param.account.sequenceNumber(),
        };
        param.account.incrementSequenceNumber();
        pool.runTask<SendLogTx>(linearized, (err, result) => {
          const end = process.hrtime(start);
          console.log(`Task finished in ${end[1] / 1000000}ms`, err, result);
          if (++finished === totalTasks) pool.close();
        })
      }
      sent += 1;
    }
    const end = process.hrtime(start);
    console.log(`Task finished in ${end[1] / 1000000}ms`);
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
  if(period === 0){
    return
  }
  return new Promise((resolve) => {
    setTimeout(resolve, period);
  });
}

main().catch(console.error);
