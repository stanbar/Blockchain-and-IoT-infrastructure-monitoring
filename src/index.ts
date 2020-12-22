import {
  Config,
  Operation,
  Keypair,
  Server,
  TransactionBuilder,
} from "stellar-sdk";

const { NETWORK_PASSPHRASE, HORIZON_SERVER_URL } = process.env;
if (!NETWORK_PASSPHRASE) {
  throw Error("NETWORK_PASSPHRASE must be defined");
}
if (!HORIZON_SERVER_URL) {
  throw Error("HORIZON_SERVER_URL must be defined");
}
console.log({ NETWORK_PASSPHRASE, HORIZON_SERVER_URL });
Config.setAllowHttp(true);
const masterKeypair = Keypair.master(NETWORK_PASSPHRASE);
const server = new Server(HORIZON_SERVER_URL, { allowHttp: true });
console.log({
  newKeypair: {
    public: masterKeypair.publicKey(),
    secret: masterKeypair.secret(),
  },
});

async function defaultOptions(): Promise<TransactionBuilder.TransactionBuilderOptions> {
  console.log("Loadiing timebounds");
  const timebounds = await server.fetchTimebounds(10);
  console.log("Loaded timebounds");
  console.log("Loadiing fee");
  const fee = await server.fetchBaseFee();
  console.log("Loaded basefee");
  return {
    networkPassphrase: NETWORK_PASSPHRASE,
    fee: `${fee}`,
    timebounds,
  };
}

async function createAccount(newKeypair: Keypair) {
  console.log("Loading master account");
  const masterAccount = await server.loadAccount(masterKeypair.publicKey());
  console.log("Loaded master account");
  const tx = new TransactionBuilder(masterAccount, await defaultOptions())
    // Create distribution account
    .addOperation(
      Operation.createAccount({
        destination: newKeypair.publicKey(),
        startingBalance: `${10 ** 7}`, // TODO calculate exactly
      })
    )
    .build();
  tx.sign(masterKeypair);
  console.log("Submitting transaction");
  const response = await server.submitTransaction(tx);
  console.log({ response });
}

const newKeypair = Keypair.random();
console.log({
  newKeypair: { public: newKeypair.publicKey(), secret: newKeypair.secret() },
});
createAccount(newKeypair).catch((err) => {
  console.error(err?.response?.data?.extras?.result_codes)
});
