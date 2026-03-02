import { NextResponse } from "next/server";
import { createWalletClient, createPublicClient, http, keccak256, toBytes } from "viem";
import { privateKeyToAccount } from "viem/accounts";
import { hardhat } from "viem/chains";
import fs from "fs";
import path from "path";

// Load compiled artifact
function loadArtifact() {
  const artifactPath = path.join(process.cwd(), "..", "artifacts", "SmartGridGW.json");
  return JSON.parse(fs.readFileSync(artifactPath, "utf8"));
}

const PRIVATE_KEYS = [
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", // admin
  "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d", // meter1
  "0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a", // meter2
  "0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6", // committee0
  "0x47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a", // committee1
  "0x8b3a350cf5c34c9194ca85829a2df0ec3153be0318b5e2d3348e872092edffba", // committee2
] as const;

export async function POST() {
  try {
    const artifact = loadArtifact();
    const accounts = PRIVATE_KEYS.map((pk) => privateKeyToAccount(pk));
    const [admin, , , committee0, committee1, committee2] = accounts;

    const walletC = createWalletClient({
      chain: hardhat,
      transport: http(),
      account: admin,
    });

    const publicC = createPublicClient({
      chain: hardhat,
      transport: http(),
    });

    const meterRoot = keccak256(toBytes("meter-root-v1"));
    const keyCommitment = keccak256(toBytes("committee-key-v1"));

    const hash = await walletC.deployContract({
      abi: artifact.abi,
      bytecode: artifact.bytecode as `0x${string}`,
      args: [
        meterRoot,
        3,  // N
        2,  // T
        keyCommitment,
        [committee0.address, committee1.address, committee2.address],
        1000000000000000n, // 0.001 ETH tariff
        60n,   // epoch duration
        120n,  // report deadline
        300n,  // decryption deadline
      ],
    });

    const receipt = await publicC.waitForTransactionReceipt({ hash });

    return NextResponse.json({
      address: receipt.contractAddress,
      txHash: hash,
      blockNumber: Number(receipt.blockNumber),
    });
  } catch (e: any) {
    return NextResponse.json({ error: e.message }, { status: 500 });
  }
}
