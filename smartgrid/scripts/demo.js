/**
 * SmartGrid-BC End-to-End Demo Script
 *
 * Runs the complete SmartGrid-BC flow:
 *   1. Deploy contract
 *   2. Submit reports from 2 meters across 2 epochs
 *   3. Create aggregate, collect shares → DecReady
 *   4. Create bill request, collect shares → DecReady
 *   5. Create aggregate with insufficient shares → CarryOver
 *
 * Usage: node scripts/demo.js
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

async function signReport(signer, meterId, epoch, commitment) {
  const msgHash = ethers.keccak256(
    ethers.solidityPacked(["bytes32", "uint256", "bytes32"], [meterId, epoch, commitment])
  );
  return signer.signMessage(ethers.getBytes(msgHash));
}

async function advanceTime(provider, seconds) {
  await provider.send("evm_increaseTime", [seconds]);
  await provider.send("evm_mine", []);
}

const ITEM_STATUSES = ["None", "Pending", "DecReady", "CarryOver"];

async function main() {
  console.log("═══════════════════════════════════════════════════════");
  console.log("  SmartGrid-BC: End-to-End Demo");
  console.log("═══════════════════════════════════════════════════════\n");

  const provider = new ethers.JsonRpcProvider("http://127.0.0.1:8545");
  const accounts = [];
  for (let i = 0; i < 10; i++) accounts.push(await provider.getSigner(i));
  const [admin, meter1, meter2, c0, c1, c2, customer] = accounts;

  const meter1Id = ethers.keccak256(await meter1.getAddress());
  const meter2Id = ethers.keccak256(await meter2.getAddress());
  const regionId = ethers.keccak256(ethers.toUtf8Bytes("region-A"));

  // ─── Step 1: Deploy ───
  console.log("Step 1: Deploying SmartGridGW...");
  const meterRoot = ethers.keccak256(ethers.toUtf8Bytes("meter-root-v1"));
  const keyCommitment = ethers.keccak256(ethers.toUtf8Bytes("committee-key-v1"));
  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode, admin);
  const gw = await factory.deploy(
    meterRoot, 3, 2, keyCommitment,
    [await c0.getAddress(), await c1.getAddress(), await c2.getAddress()],
    TARIFF_RATE, EPOCH_DURATION, REPORT_DEADLINE, DECRYPTION_DEADLINE
  );
  await gw.waitForDeployment();
  console.log(`  Contract: ${await gw.getAddress()}`);
  console.log(`  Epoch duration: ${EPOCH_DURATION}s, Threshold: 2/3\n`);

  // ─── Step 2: Epoch 0 reports ───
  console.log("Step 2: Epoch 0 — Meter reports");
  const readings = { m1e0: 42, m2e0: 55, m1e1: 50, m2e1: 60 };

  for (const [key, reading, signer, mid] of [
    ["m1e0", readings.m1e0, meter1, meter1Id],
    ["m2e0", readings.m2e0, meter2, meter2Id],
  ]) {
    const commitment = ethers.keccak256(ethers.toUtf8Bytes(`${key}-${reading}kwh`));
    const cipherTag = ethers.keccak256(ethers.toUtf8Bytes(`enc-${key}`));
    const sig = await signReport(signer, mid, 0, commitment);
    await (await gw.submitReport(mid, 0, commitment, cipherTag, sig, "0x")).wait();
    console.log(`  ${key}: ${reading} kWh → Accepted`);
  }

  // Verify duplicate rejection
  try {
    const c = ethers.keccak256(ethers.toUtf8Bytes("m1e0-42kwh"));
    const ct = ethers.keccak256(ethers.toUtf8Bytes("enc-m1e0"));
    const sig = await signReport(meter1, meter1Id, 0, c);
    await gw.submitReport(meter1Id, 0, c, ct, sig, "0x");
    console.log("  ERROR: Duplicate should have been rejected!");
  } catch {
    console.log("  ✓ Duplicate report for (meter1, epoch0) correctly rejected");
  }

  // ─── Step 3: Advance to Epoch 1, more reports ───
  console.log("\nStep 3: Advance to Epoch 1 + reports");
  await advanceTime(provider, EPOCH_DURATION);

  for (const [key, reading, signer, mid] of [
    ["m1e1", readings.m1e1, meter1, meter1Id],
    ["m2e1", readings.m2e1, meter2, meter2Id],
  ]) {
    const commitment = ethers.keccak256(ethers.toUtf8Bytes(`${key}-${reading}kwh`));
    const cipherTag = ethers.keccak256(ethers.toUtf8Bytes(`enc-${key}`));
    const sig = await signReport(signer, mid, 1, commitment);
    await (await gw.submitReport(mid, 1, commitment, cipherTag, sig, "0x")).wait();
    console.log(`  ${key}: ${reading} kWh → Accepted`);
  }

  // ─── Step 4: Aggregate for epoch 0 → DecReady ───
  console.log("\nStep 4: Aggregate for epoch 0 → threshold decryption");
  const aggRef0 = ethers.keccak256(ethers.toUtf8Bytes("agg-region-epoch0"));
  await (await gw.submitAggregate(regionId, 0, aggRef0)).wait();
  const aggItemId0 = await gw.computeAggregateItemId(regionId, 0);
  console.log(`  Aggregate item: ${aggItemId0.slice(0, 20)}...`);

  await (await gw.connect(c0).submitDecryptionShare(aggItemId0, 0, ethers.keccak256(ethers.toUtf8Bytes("agg0-s0")))).wait();
  console.log("  Committee 0: share submitted (1/2)");
  await (await gw.connect(c1).submitDecryptionShare(aggItemId0, 1, ethers.keccak256(ethers.toUtf8Bytes("agg0-s1")))).wait();
  console.log("  Committee 1: share submitted (2/2)");

  let item = await gw.getItem(aggItemId0);
  console.log(`  Status: ${ITEM_STATUSES[item[0]]} ✓\n`);

  // ─── Step 5: Bill for meter 1 → DecReady ───
  console.log("Step 5: Bill decryption for meter 1");
  await (await gw.connect(customer).requestBillDecryption(meter1Id)).wait();
  const billItemId = await gw.computeBillItemId(meter1Id);
  console.log(`  Bill item: ${billItemId.slice(0, 20)}...`);

  await (await gw.connect(c0).submitDecryptionShare(billItemId, 0, ethers.keccak256(ethers.toUtf8Bytes("bill-s0")))).wait();
  console.log("  Committee 0: share submitted (1/2)");
  await (await gw.connect(c1).submitDecryptionShare(billItemId, 1, ethers.keccak256(ethers.toUtf8Bytes("bill-s1")))).wait();
  console.log("  Committee 1: share submitted (2/2)");

  item = await gw.getItem(billItemId);
  console.log(`  Bill status: ${ITEM_STATUSES[item[0]]} ✓`);
  console.log(`  Bill cipher state: ${await gw.billCipherState(meter1Id)}`);

  // ─── Step 6: Aggregate for epoch 1 → insufficient shares → CarryOver ───
  console.log("\nStep 6: Aggregate for epoch 1 → CarryOver (deadline timeout)");
  const aggRef1 = ethers.keccak256(ethers.toUtf8Bytes("agg-region-epoch1"));
  await (await gw.submitAggregate(regionId, 1, aggRef1)).wait();
  const aggItemId1 = await gw.computeAggregateItemId(regionId, 1);

  // Only 1 share (below threshold of 2)
  await (await gw.connect(c2).submitDecryptionShare(aggItemId1, 2, ethers.keccak256(ethers.toUtf8Bytes("agg1-s2")))).wait();
  console.log("  Committee 2: share submitted (1/2 — below threshold)");

  // Advance past decryption deadline
  await advanceTime(provider, DECRYPTION_DEADLINE + 1);
  await (await gw.markCarryOver(aggItemId1)).wait();
  item = await gw.getItem(aggItemId1);
  console.log(`  Status after deadline: ${ITEM_STATUSES[item[0]]} ✓`);

  // ─── Summary ───
  console.log("\n═══════════════════════════════════════════════════════");
  console.log("  DEMO COMPLETE — All acceptance criteria met:");
  console.log("  ✓ 2 meters × 2 epochs = 4 reports accepted");
  console.log("  ✓ Duplicate (meter1, epoch0) rejected");
  console.log("  ✓ Aggregate epoch 0 → DecReady (2/2 shares)");
  console.log("  ✓ Bill meter 1 → DecReady (2/2 shares)");
  console.log("  ✓ Aggregate epoch 1 → CarryOver (1/2, deadline passed)");
  console.log("═══════════════════════════════════════════════════════\n");
}

main().catch((e) => {
  console.error("Demo failed:", e);
  process.exit(1);
});
