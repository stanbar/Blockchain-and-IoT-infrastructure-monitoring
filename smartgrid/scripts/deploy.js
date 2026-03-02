/**
 * Deploy SmartGridGW to a local Hardhat node.
 * Outputs deployment info to artifacts/deployment.json
 *
 * Usage: node scripts/deploy.js
 *   (requires `npx hardhat node` running on :8545)
 */
const { ethers } = require("ethers");
const fs = require("fs");
const path = require("path");

const artifact = JSON.parse(
  fs.readFileSync(path.join(__dirname, "..", "artifacts", "SmartGridGW.json"), "utf8")
);

const EPOCH_DURATION = 60;
const REPORT_DEADLINE = 120;
const DECRYPTION_DEADLINE = 300;
const TARIFF_RATE = ethers.parseEther("0.001");

async function main() {
  const provider = new ethers.JsonRpcProvider("http://127.0.0.1:8545");
  const accounts = [];
  for (let i = 0; i < 10; i++) {
    accounts.push(await provider.getSigner(i));
  }

  const [admin, meter1, meter2, committee0, committee1, committee2, customer] = accounts;

  const meterRoot = ethers.keccak256(ethers.toUtf8Bytes("meter-root-v1"));
  const keyCommitment = ethers.keccak256(ethers.toUtf8Bytes("committee-key-v1"));

  console.log("Deploying SmartGridGW...");
  console.log(`  Admin:      ${await admin.getAddress()}`);
  console.log(`  Committee:  [${await committee0.getAddress()}, ${await committee1.getAddress()}, ${await committee2.getAddress()}]`);
  console.log(`  Threshold:  2 of 3`);

  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode, admin);
  const contract = await factory.deploy(
    meterRoot,
    3, // N
    2, // T
    keyCommitment,
    [await committee0.getAddress(), await committee1.getAddress(), await committee2.getAddress()],
    TARIFF_RATE,
    EPOCH_DURATION,
    REPORT_DEADLINE,
    DECRYPTION_DEADLINE
  );
  await contract.waitForDeployment();
  const contractAddress = await contract.getAddress();

  console.log(`\n  Contract deployed at: ${contractAddress}`);
  console.log(`  Epoch duration: ${EPOCH_DURATION}s`);
  console.log(`  Report deadline: ${REPORT_DEADLINE}s`);
  console.log(`  Decryption deadline: ${DECRYPTION_DEADLINE}s`);

  const deployment = {
    contractAddress,
    network: "localhost",
    chainId: 31337,
    epochDuration: EPOCH_DURATION,
    reportDeadline: REPORT_DEADLINE,
    decryptionDeadline: DECRYPTION_DEADLINE,
    roles: {
      admin: await admin.getAddress(),
      meters: [
        { id: ethers.keccak256(await meter1.getAddress()), address: await meter1.getAddress(), label: "Meter 1" },
        { id: ethers.keccak256(await meter2.getAddress()), address: await meter2.getAddress(), label: "Meter 2" },
      ],
      committee: [
        { index: 0, address: await committee0.getAddress() },
        { index: 1, address: await committee1.getAddress() },
        { index: 2, address: await committee2.getAddress() },
      ],
      customer: await customer.getAddress(),
    },
    deployTimestamp: Number(await contract.deployTimestamp()),
  };

  const outPath = path.join(__dirname, "..", "artifacts", "deployment.json");
  fs.writeFileSync(outPath, JSON.stringify(deployment, null, 2));
  console.log(`\n  Deployment info saved to: ${outPath}`);
}

main().catch((e) => {
  console.error("Deploy failed:", e);
  process.exit(1);
});
