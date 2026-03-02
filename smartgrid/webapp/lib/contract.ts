import {
  createPublicClient,
  createWalletClient,
  http,
  keccak256,
  toBytes,
  encodePacked,
  type Address,
  type Hex,
  type WalletClient,
  type PublicClient,
  type Account,
} from "viem";
import { hardhat } from "viem/chains";

// Contract ABI — matches SmartGridGW.sol
export const SMARTGRID_ABI = [
  // Views
  { type: "function", name: "admin", inputs: [], outputs: [{ type: "address" }], stateMutability: "view" },
  { type: "function", name: "meterRoot", inputs: [], outputs: [{ type: "bytes32" }], stateMutability: "view" },
  { type: "function", name: "committeeN", inputs: [], outputs: [{ type: "uint8" }], stateMutability: "view" },
  { type: "function", name: "committeeT", inputs: [], outputs: [{ type: "uint8" }], stateMutability: "view" },
  { type: "function", name: "committeeKeyCommitment", inputs: [], outputs: [{ type: "bytes32" }], stateMutability: "view" },
  { type: "function", name: "committeeMembers", inputs: [{ type: "uint8" }], outputs: [{ type: "address" }], stateMutability: "view" },
  { type: "function", name: "tariffRate", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "epochDuration", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "reportDeadline", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "decryptionDeadline", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "deployTimestamp", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "currentEpoch", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "billCipherState", inputs: [{ type: "bytes32" }], outputs: [{ type: "bytes32" }], stateMutability: "view" },
  { type: "function", name: "meterLastEpoch", inputs: [{ type: "bytes32" }], outputs: [{ type: "uint256" }], stateMutability: "view" },
  {
    type: "function", name: "getReportStatus",
    inputs: [{ type: "bytes32" }, { type: "uint256" }],
    outputs: [{ type: "uint8" }],
    stateMutability: "view",
  },
  {
    type: "function", name: "getReportDetails",
    inputs: [{ type: "bytes32" }, { type: "uint256" }],
    outputs: [{ type: "uint8", name: "status" }, { type: "bytes32", name: "commitment" }, { type: "bytes32", name: "ciphertextTag" }],
    stateMutability: "view",
  },
  {
    type: "function", name: "getItem",
    inputs: [{ type: "bytes32" }],
    outputs: [
      { type: "uint8", name: "status" },
      { type: "bytes32", name: "ciphertextRef" },
      { type: "uint256", name: "createdAt" },
      { type: "uint8", name: "sharesReceived" },
    ],
    stateMutability: "view",
  },
  {
    type: "function", name: "getItemShare",
    inputs: [{ type: "bytes32" }, { type: "uint8" }],
    outputs: [{ type: "bytes32" }],
    stateMutability: "view",
  },
  {
    type: "function", name: "computeAggregateItemId",
    inputs: [{ type: "bytes32" }, { type: "uint256" }],
    outputs: [{ type: "bytes32" }],
    stateMutability: "pure",
  },
  {
    type: "function", name: "computeBillItemId",
    inputs: [{ type: "bytes32" }],
    outputs: [{ type: "bytes32" }],
    stateMutability: "pure",
  },
  // Mutators
  {
    type: "function", name: "submitReport",
    inputs: [
      { type: "bytes32", name: "meterId" },
      { type: "uint256", name: "epoch" },
      { type: "bytes32", name: "commitment" },
      { type: "bytes32", name: "ciphertextTag" },
      { type: "bytes", name: "signature" },
      { type: "bytes", name: "proof" },
    ],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function", name: "submitAggregate",
    inputs: [
      { type: "bytes32", name: "regionId" },
      { type: "uint256", name: "epoch" },
      { type: "bytes32", name: "ciphertextRef" },
    ],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function", name: "requestBillDecryption",
    inputs: [{ type: "bytes32", name: "meterId" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function", name: "submitDecryptionShare",
    inputs: [
      { type: "bytes32", name: "itemId" },
      { type: "uint8", name: "memberIndex" },
      { type: "bytes32", name: "share" },
    ],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function", name: "markCarryOver",
    inputs: [{ type: "bytes32", name: "itemId" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function", name: "updateMeterRoot",
    inputs: [{ type: "bytes32" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  {
    type: "function", name: "updateParameters",
    inputs: [{ type: "uint256" }, { type: "uint256" }],
    outputs: [],
    stateMutability: "nonpayable",
  },
  // Events
  { type: "event", name: "ReportAccepted", inputs: [{ type: "bytes32", indexed: true, name: "meterId" }, { type: "uint256", indexed: true, name: "epoch" }, { type: "bytes32", name: "commitment" }, { type: "bytes32", name: "ciphertextTag" }] },
  { type: "event", name: "AggregateCreated", inputs: [{ type: "bytes32", indexed: true, name: "itemId" }, { type: "bytes32", indexed: true, name: "regionId" }, { type: "uint256", indexed: true, name: "epoch" }, { type: "bytes32", name: "ciphertextRef" }] },
  { type: "event", name: "BillItemCreated", inputs: [{ type: "bytes32", indexed: true, name: "itemId" }, { type: "bytes32", indexed: true, name: "meterId" }, { type: "bytes32", name: "ciphertextRef" }] },
  { type: "event", name: "DecShareSubmitted", inputs: [{ type: "bytes32", indexed: true, name: "itemId" }, { type: "uint8", indexed: true, name: "memberIndex" }, { type: "bytes32", name: "share" }] },
  { type: "event", name: "ItemDecReady", inputs: [{ type: "bytes32", indexed: true, name: "itemId" }] },
  { type: "event", name: "ItemCarryOver", inputs: [{ type: "bytes32", indexed: true, name: "itemId" }] },
] as const;

// Hardhat default accounts (from mnemonic "test test test test test test test test test test test junk")
export const HARDHAT_ACCOUNTS = [
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
  "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
  "0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a",
  "0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6",
  "0x47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a",
  "0x8b3a350cf5c34c9194ca85829a2df0ec3153be0318b5e2d3348e872092edffba",
  "0x92db14e403b83dfe3df233f83dfa3a0d7096f21ca9b0d6d6b8d88b2b4ec1564e",
  "0xdbda1821b80551c9d65939329250298aa3472ba22feea921c0cf5d620ea67b97",
  "0x2a871d0798f97d79848a013d4936a73bf4cc922c825d33c1cf7073dff6d409c6",
  "0xa267530f49f8280200edf313ee7af6b827f2a8bce2897751d06a843f644967b1",
] as const;

export const ROLE_LABELS: Record<number, string> = {
  0: "Admin / Regulator",
  1: "Meter 1",
  2: "Meter 2",
  3: "Committee Member 0",
  4: "Committee Member 1",
  5: "Committee Member 2",
  6: "Customer",
};

export function getPublicClient() {
  return createPublicClient({ chain: hardhat, transport: http() });
}

export function getWalletClient(accountIndex: number) {
  const privateKey = HARDHAT_ACCOUNTS[accountIndex];
  return createWalletClient({
    chain: hardhat,
    transport: http(),
    account: privateKeyToAccount(privateKey),
  });
}

// Inline privateKey -> account to avoid dependency on viem/accounts in SSR
function privateKeyToAccount(privateKey: Hex): Account {
  // We'll use viem's account type
  const { privateKeyToAccount: pk2a } = require("viem/accounts");
  return pk2a(privateKey);
}

export function computeMeterId(address: Address): Hex {
  return keccak256(address as Hex);
}

export const REGION_ID = keccak256(toBytes("region-A"));

export const ITEM_STATUS_LABELS: Record<number, string> = {
  0: "None",
  1: "Pending",
  2: "DecReady",
  3: "CarryOver",
};

export const REPORT_STATUS_LABELS: Record<number, string> = {
  0: "None",
  1: "Accepted",
};
