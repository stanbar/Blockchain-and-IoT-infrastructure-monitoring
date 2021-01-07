import {TransactionBuilder} from "stellar-sdk";

const { NETWORK_PASSPHRASE, } = process.env; 
if (!NETWORK_PASSPHRASE) {
  throw new Error("NETWORK_PASSPHRASE must be defined");
}

export function defaultOptions(): TransactionBuilder.TransactionBuilderOptions {
  /* console.log("Loadiing timebounds"); */
  /* const timebounds = await server.fetchTimebounds(10); */
  /* console.log("Loaded timebounds"); */
  return {
    networkPassphrase: NETWORK_PASSPHRASE,
    fee: "1",
  };
}

export type SendLogTx = {
  deviceId: number;
  index: number;
  server: string;
  batchAddress: string;
  deviceSecret: string;
  accountId: string;
  accountSeq: string;
};

export function submittingMessage(index: number, deviceId: number){
  console.log(
    `[${format2Digit(index)}${format3Digit(
      deviceId
    )}] Submitting tx`
  );
}
export function successMessage(index: number, deviceId: number){
  console.log(
    `[${format2Digit(index)}${format3Digit(
      deviceId
    )}] Successfully send`
  );
}

const format3Digit = (text: any) => formatDigit(3)(text);
const format2Digit = (text: any) => formatDigit(2)(text);
const formatDigit = (digits: number) => (text: any) =>
  text.toLocaleString(undefined, {
    minimumIntegerDigits: digits,
    useGrouping: false,
  });
