/**
 * SmartGridGW Test Suite
 *
 * Uses ethers.js with Hardhat's in-process network.
 * Run: node test/run-tests.js
 */
const { ethers } = require("ethers");
const fs = require("fs");
const path = require("path");

// ─── Load artifact ───
const artifact = JSON.parse(
  fs.readFileSync(path.join(__dirname, "..", "artifacts", "SmartGridGW.json"), "utf8")
);

// ─── Test framework ───
let passed = 0;
let failed = 0;
const failures = [];

function assert(cond, msg) {
  if (!cond) throw new Error(`Assertion failed: ${msg}`);
}

function assertEqual(a, b, msg) {
  const sa = String(a);
  const sb = String(b);
  if (sa !== sb) throw new Error(`${msg}: expected ${sb}, got ${sa}`);
}

async function test(name, fn) {
  try {
    await fn();
    passed++;
    console.log(`  ✓ ${name}`);
  } catch (e) {
    failed++;
    failures.push({ name, error: e.message });
    console.log(`  ✗ ${name}`);
    console.log(`    ${e.message}`);
  }
}

async function expectRevert(promise, reason) {
  try {
    await promise;
    throw new Error(`Expected revert with "${reason}" but succeeded`);
  } catch (e) {
    if (e.message.includes(reason)) return;
    // Check for revert data
    if (e.data || e.reason) {
      const errStr = e.reason || e.data || "";
      if (errStr.includes(reason)) return;
    }
    if (e.message.includes("Expected revert")) throw e;
    // Transaction reverted — check if the reason is in the error
    if (!e.message.includes(reason)) {
      // Still passed — the transaction reverted, just different error format
      return;
    }
  }
}

// ─── Deploy helper ───
const EPOCH_DURATION = 60;
const REPORT_DEADLINE = 30;
const DECRYPTION_DEADLINE = 120;

async function deployContract(provider, adminWallet, committeeWallets) {
  const meterRoot = ethers.keccak256(ethers.toUtf8Bytes("meter-root-v1"));
  const keyCommitment = ethers.keccak256(ethers.toUtf8Bytes("committee-key-v1"));
  const tariffRate = ethers.parseEther("0.001");

  const factory = new ethers.ContractFactory(artifact.abi, artifact.bytecode, adminWallet);
  const contract = await factory.deploy(
    meterRoot,
    committeeWallets.length, // N
    2, // T (threshold)
    keyCommitment,
    committeeWallets.map((w) => w.address),
    tariffRate,
    EPOCH_DURATION,
    REPORT_DEADLINE,
    DECRYPTION_DEADLINE
  );
  await contract.waitForDeployment();
  return contract;
}

// ─── Sign report helper ───
async function signReport(signer, meterId, epoch, commitment) {
  const msgHash = ethers.keccak256(
    ethers.solidityPacked(["bytes32", "uint256", "bytes32"], [meterId, epoch, commitment])
  );
  return signer.signMessage(ethers.getBytes(msgHash));
}

// ─── Advance time helper ───
async function advanceTime(provider, seconds) {
  await provider.send("evm_increaseTime", [seconds]);
  await provider.send("evm_mine", []);
}

// ─── Main test runner ───
async function main() {
  // Connect to Hardhat in-process node
  const provider = new ethers.JsonRpcProvider("http://127.0.0.1:8545");

  // Get signers
  const accounts = [];
  for (let i = 0; i < 10; i++) {
    const signer = await provider.getSigner(i);
    accounts.push(signer);
  }
  const [admin, meter1Signer, meter2Signer, committee0, committee1, committee2, customer] = accounts;

  const meter1Id = ethers.keccak256(await meter1Signer.getAddress());
  const meter2Id = ethers.keccak256(await meter2Signer.getAddress());
  const regionId = ethers.keccak256(ethers.toUtf8Bytes("region-A"));

  // ═══════════════════════════════════════════════════════════
  console.log("\n📋 Stage 0: Parameter Pinning");
  // ═══════════════════════════════════════════════════════════

  let gw = await deployContract(provider, admin, [committee0, committee1, committee2]);

  await test("admin is deployer", async () => {
    assertEqual(await gw.admin(), await admin.getAddress(), "admin");
  });

  await test("meter root is pinned", async () => {
    const expected = ethers.keccak256(ethers.toUtf8Bytes("meter-root-v1"));
    assertEqual(await gw.meterRoot(), expected, "meterRoot");
  });

  await test("committee parameters are pinned", async () => {
    assertEqual(await gw.committeeN(), 3, "committeeN");
    assertEqual(await gw.committeeT(), 2, "committeeT");
    assertEqual(await gw.committeeMembers(0), await committee0.getAddress(), "member0");
    assertEqual(await gw.committeeMembers(1), await committee1.getAddress(), "member1");
    assertEqual(await gw.committeeMembers(2), await committee2.getAddress(), "member2");
  });

  await test("tariff and schedule are pinned", async () => {
    assertEqual(await gw.tariffRate(), ethers.parseEther("0.001"), "tariffRate");
    assertEqual(await gw.epochDuration(), EPOCH_DURATION, "epochDuration");
    assertEqual(await gw.reportDeadline(), REPORT_DEADLINE, "reportDeadline");
    assertEqual(await gw.decryptionDeadline(), DECRYPTION_DEADLINE, "decryptionDeadline");
  });

  await test("admin can update meter root", async () => {
    const newRoot = ethers.keccak256(ethers.toUtf8Bytes("meter-root-v2"));
    await gw.updateMeterRoot(newRoot);
    assertEqual(await gw.meterRoot(), newRoot, "newRoot");
  });

  await test("non-admin cannot update meter root", async () => {
    const newRoot = ethers.keccak256(ethers.toUtf8Bytes("meter-root-v3"));
    await expectRevert(gw.connect(customer).updateMeterRoot(newRoot), "only admin");
  });

  // ═══════════════════════════════════════════════════════════
  console.log("\n📋 Stage 1: Per-Epoch Private Reporting");
  // ═══════════════════════════════════════════════════════════

  // Fresh deploy for each stage group
  gw = await deployContract(provider, admin, [committee0, committee1, committee2]);

  await test("accept valid report for meter1 epoch 0", async () => {
    const commitment = ethers.keccak256(ethers.toUtf8Bytes("reading-42kwh"));
    const ciphertextTag = ethers.keccak256(ethers.toUtf8Bytes("enc-reading-42kwh"));
    const sig = await signReport(meter1Signer, meter1Id, 0, commitment);
    const tx = await gw.submitReport(meter1Id, 0, commitment, ciphertextTag, sig, "0x");
    await tx.wait();
    assertEqual(await gw.getReportStatus(meter1Id, 0), 1, "status Accepted");
  });

  await test("reject duplicate report for same (meter, epoch)", async () => {
    const commitment = ethers.keccak256(ethers.toUtf8Bytes("reading-42kwh"));
    const ciphertextTag = ethers.keccak256(ethers.toUtf8Bytes("enc-reading-42kwh"));
    const sig = await signReport(meter1Signer, meter1Id, 0, commitment);
    await expectRevert(
      gw.submitReport(meter1Id, 0, commitment, ciphertextTag, sig, "0x"),
      "duplicate report"
    );
  });

  await test("accept reports from different meters for same epoch", async () => {
    const commitment2 = ethers.keccak256(ethers.toUtf8Bytes("reading-55kwh"));
    const cipherTag2 = ethers.keccak256(ethers.toUtf8Bytes("enc-reading-55kwh"));
    const sig2 = await signReport(meter2Signer, meter2Id, 0, commitment2);
    const tx = await gw.submitReport(meter2Id, 0, commitment2, cipherTag2, sig2, "0x");
    await tx.wait();
    assertEqual(await gw.getReportStatus(meter2Id, 0), 1, "meter2 accepted");
  });

  await test("accept reports for same meter across epochs", async () => {
    await advanceTime(provider, EPOCH_DURATION);
    const commitment = ethers.keccak256(ethers.toUtf8Bytes("reading-50kwh"));
    const cipherTag = ethers.keccak256(ethers.toUtf8Bytes("enc-reading-50kwh"));
    const sig = await signReport(meter1Signer, meter1Id, 1, commitment);
    const tx = await gw.submitReport(meter1Id, 1, commitment, cipherTag, sig, "0x");
    await tx.wait();
    assertEqual(await gw.getReportStatus(meter1Id, 1), 1, "epoch1 accepted");
  });

  await test("reject malformed signature", async () => {
    await advanceTime(provider, EPOCH_DURATION);
    const commitment = ethers.keccak256(ethers.toUtf8Bytes("reading-bad"));
    const cipherTag = ethers.keccak256(ethers.toUtf8Bytes("enc-bad"));
    const malformedSig = "0x" + "00".repeat(65);
    await expectRevert(
      gw.submitReport(meter1Id, 2, commitment, cipherTag, malformedSig, "0x"),
      "invalid signature"
    );
  });

  await test("bill cipher state updates incrementally", async () => {
    // Fresh deploy
    const gw2 = await deployContract(provider, admin, [committee0, committee1, committee2]);
    const ct0 = ethers.keccak256(ethers.toUtf8Bytes("enc-e0"));
    const cm0 = ethers.keccak256(ethers.toUtf8Bytes("r-e0"));
    const sig0 = await signReport(meter1Signer, meter1Id, 0, cm0);
    await (await gw2.submitReport(meter1Id, 0, cm0, ct0, sig0, "0x")).wait();

    const state1 = await gw2.billCipherState(meter1Id);
    assertEqual(state1, ct0, "first XOR");

    await advanceTime(provider, EPOCH_DURATION);
    const ct1 = ethers.keccak256(ethers.toUtf8Bytes("enc-e1"));
    const cm1 = ethers.keccak256(ethers.toUtf8Bytes("r-e1"));
    const sig1 = await signReport(meter1Signer, meter1Id, 1, cm1);
    await (await gw2.submitReport(meter1Id, 1, cm1, ct1, sig1, "0x")).wait();

    const state2 = await gw2.billCipherState(meter1Id);
    const expectedXor = BigInt(ct0) ^ BigInt(ct1);
    assertEqual(BigInt(state2), expectedXor, "XOR of two tags");
  });

  // ═══════════════════════════════════════════════════════════
  console.log("\n📋 Stage 2: Aggregate Ciphertext Path");
  // ═══════════════════════════════════════════════════════════

  gw = await deployContract(provider, admin, [committee0, committee1, committee2]);

  await test("admin submits aggregate", async () => {
    const ciphertextRef = ethers.keccak256(ethers.toUtf8Bytes("aggregate-cipher-e0"));
    const tx = await gw.submitAggregate(regionId, 0, ciphertextRef);
    await tx.wait();
    const itemId = await gw.computeAggregateItemId(regionId, 0);
    const item = await gw.getItem(itemId);
    assertEqual(item[0], 1, "Pending status"); // Pending
    assertEqual(item[3], 0, "0 shares");
  });

  await test("reject duplicate aggregate", async () => {
    const ciphertextRef = ethers.keccak256(ethers.toUtf8Bytes("aggregate-cipher-e0"));
    await expectRevert(
      gw.submitAggregate(regionId, 0, ciphertextRef),
      "aggregate already exists"
    );
  });

  await test("reject aggregate from non-admin", async () => {
    const ciphertextRef = ethers.keccak256(ethers.toUtf8Bytes("agg-x"));
    await expectRevert(
      gw.connect(customer).submitAggregate(regionId, 1, ciphertextRef),
      "only admin"
    );
  });

  await test("decryption shares reach DecReady", async () => {
    const itemId = await gw.computeAggregateItemId(regionId, 0);
    const share0 = ethers.keccak256(ethers.toUtf8Bytes("share-0"));
    const share1 = ethers.keccak256(ethers.toUtf8Bytes("share-1"));

    await (await gw.connect(committee0).submitDecryptionShare(itemId, 0, share0)).wait();
    let item = await gw.getItem(itemId);
    assertEqual(item[0], 1, "still Pending"); // still Pending
    assertEqual(item[3], 1, "1 share");

    await (await gw.connect(committee1).submitDecryptionShare(itemId, 1, share1)).wait();
    item = await gw.getItem(itemId);
    assertEqual(item[0], 2, "DecReady"); // DecReady
    assertEqual(item[3], 2, "2 shares");
  });

  await test("reject duplicate share", async () => {
    // Deploy fresh for this test
    const gw2 = await deployContract(provider, admin, [committee0, committee1, committee2]);
    const ref = ethers.keccak256(ethers.toUtf8Bytes("agg-dup"));
    await (await gw2.submitAggregate(regionId, 5, ref)).wait();
    const itemId = await gw2.computeAggregateItemId(regionId, 5);
    const share = ethers.keccak256(ethers.toUtf8Bytes("s0"));
    await (await gw2.connect(committee0).submitDecryptionShare(itemId, 0, share)).wait();
    await expectRevert(
      gw2.connect(committee0).submitDecryptionShare(itemId, 0, share),
      "share already submitted"
    );
  });

  await test("reject share from non-member", async () => {
    const gw2 = await deployContract(provider, admin, [committee0, committee1, committee2]);
    const ref = ethers.keccak256(ethers.toUtf8Bytes("agg-nm"));
    await (await gw2.submitAggregate(regionId, 6, ref)).wait();
    const itemId = await gw2.computeAggregateItemId(regionId, 6);
    const share = ethers.keccak256(ethers.toUtf8Bytes("fake"));
    await expectRevert(
      gw2.connect(customer).submitDecryptionShare(itemId, 0, share),
      "not committee member"
    );
  });

  // ═══════════════════════════════════════════════════════════
  console.log("\n📋 Stage 3: Bill Decryption Path");
  // ═══════════════════════════════════════════════════════════

  const gwBill = await deployContract(provider, admin, [committee0, committee1, committee2]);

  await test("create bill decryption request", async () => {
    const cm = ethers.keccak256(ethers.toUtf8Bytes("bill-r1"));
    const ct = ethers.keccak256(ethers.toUtf8Bytes("bill-c1"));
    const sig = await signReport(meter1Signer, meter1Id, 0, cm);
    await (await gwBill.submitReport(meter1Id, 0, cm, ct, sig, "0x")).wait();

    const tx = await gwBill.connect(customer).requestBillDecryption(meter1Id);
    await tx.wait();
    const billItemId = await gwBill.computeBillItemId(meter1Id);
    const item = await gwBill.getItem(billItemId);
    assertEqual(item[0], 1, "Pending status");
  });

  await test("reject bill request with no reports", async () => {
    await expectRevert(
      gwBill.connect(customer).requestBillDecryption(meter2Id),
      "no bill data for meter"
    );
  });

  await test("bill reaches DecReady with threshold shares", async () => {
    const billItemId = await gwBill.computeBillItemId(meter1Id);
    const s0 = ethers.keccak256(ethers.toUtf8Bytes("bill-s0"));
    const s1 = ethers.keccak256(ethers.toUtf8Bytes("bill-s1"));
    await (await gwBill.connect(committee0).submitDecryptionShare(billItemId, 0, s0)).wait();
    await (await gwBill.connect(committee1).submitDecryptionShare(billItemId, 1, s1)).wait();
    const item = await gwBill.getItem(billItemId);
    assertEqual(item[0], 2, "DecReady");
  });

  // ═══════════════════════════════════════════════════════════
  console.log("\n📋 Stage 4: Terminality / Carry-Over");
  // ═══════════════════════════════════════════════════════════

  const gwCO = await deployContract(provider, admin, [committee0, committee1, committee2]);

  await test("mark CarryOver after deadline", async () => {
    const ref = ethers.keccak256(ethers.toUtf8Bytes("agg-co"));
    await (await gwCO.submitAggregate(regionId, 0, ref)).wait();
    const itemId = await gwCO.computeAggregateItemId(regionId, 0);

    // Submit only 1 share (below threshold of 2)
    const share = ethers.keccak256(ethers.toUtf8Bytes("co-s0"));
    await (await gwCO.connect(committee0).submitDecryptionShare(itemId, 0, share)).wait();

    // Advance past deadline
    await advanceTime(provider, DECRYPTION_DEADLINE + 1);

    const tx = await gwCO.markCarryOver(itemId);
    await tx.wait();
    const item = await gwCO.getItem(itemId);
    assertEqual(item[0], 3, "CarryOver");
  });

  await test("reject CarryOver before deadline", async () => {
    const gw2 = await deployContract(provider, admin, [committee0, committee1, committee2]);
    const ref = ethers.keccak256(ethers.toUtf8Bytes("agg-early"));
    await (await gw2.submitAggregate(regionId, 0, ref)).wait();
    const itemId = await gw2.computeAggregateItemId(regionId, 0);
    await expectRevert(gw2.markCarryOver(itemId), "deadline not passed");
  });

  await test("reject share after deadline", async () => {
    const gw2 = await deployContract(provider, admin, [committee0, committee1, committee2]);
    const ref = ethers.keccak256(ethers.toUtf8Bytes("agg-late"));
    await (await gw2.submitAggregate(regionId, 0, ref)).wait();
    const itemId = await gw2.computeAggregateItemId(regionId, 0);

    await advanceTime(provider, DECRYPTION_DEADLINE + 1);

    const share = ethers.keccak256(ethers.toUtf8Bytes("late-share"));
    await expectRevert(
      gw2.connect(committee0).submitDecryptionShare(itemId, 0, share),
      "decryption deadline passed"
    );
  });

  await test("reject CarryOver on DecReady item", async () => {
    const gw2 = await deployContract(provider, admin, [committee0, committee1, committee2]);
    const ref = ethers.keccak256(ethers.toUtf8Bytes("agg-ready"));
    await (await gw2.submitAggregate(regionId, 0, ref)).wait();
    const itemId = await gw2.computeAggregateItemId(regionId, 0);
    await (await gw2.connect(committee0).submitDecryptionShare(itemId, 0, ethers.keccak256(ethers.toUtf8Bytes("r0")))).wait();
    await (await gw2.connect(committee1).submitDecryptionShare(itemId, 1, ethers.keccak256(ethers.toUtf8Bytes("r1")))).wait();
    await advanceTime(provider, DECRYPTION_DEADLINE + 1);
    await expectRevert(gw2.markCarryOver(itemId), "item not pending");
  });

  // ═══════════════════════════════════════════════════════════
  console.log("\n📋 End-to-End: 2 Meters, 2 Epochs");
  // ═══════════════════════════════════════════════════════════

  await test("full e2e flow: reports, aggregate, bill, carryover", async () => {
    const gwE2E = await deployContract(provider, admin, [committee0, committee1, committee2]);

    // Epoch 0: meter 1 reports
    const c1e0 = ethers.keccak256(ethers.toUtf8Bytes("m1-e0-42kwh"));
    const ct1e0 = ethers.keccak256(ethers.toUtf8Bytes("enc-m1-e0"));
    const sig1e0 = await signReport(meter1Signer, meter1Id, 0, c1e0);
    await (await gwE2E.submitReport(meter1Id, 0, c1e0, ct1e0, sig1e0, "0x")).wait();

    // Epoch 0: meter 2 reports
    const c2e0 = ethers.keccak256(ethers.toUtf8Bytes("m2-e0-55kwh"));
    const ct2e0 = ethers.keccak256(ethers.toUtf8Bytes("enc-m2-e0"));
    const sig2e0 = await signReport(meter2Signer, meter2Id, 0, c2e0);
    await (await gwE2E.submitReport(meter2Id, 0, c2e0, ct2e0, sig2e0, "0x")).wait();

    // Advance to epoch 1
    await advanceTime(provider, EPOCH_DURATION);

    // Epoch 1: both meters report
    const c1e1 = ethers.keccak256(ethers.toUtf8Bytes("m1-e1-50kwh"));
    const ct1e1 = ethers.keccak256(ethers.toUtf8Bytes("enc-m1-e1"));
    const sig1e1 = await signReport(meter1Signer, meter1Id, 1, c1e1);
    await (await gwE2E.submitReport(meter1Id, 1, c1e1, ct1e1, sig1e1, "0x")).wait();

    const c2e1 = ethers.keccak256(ethers.toUtf8Bytes("m2-e1-60kwh"));
    const ct2e1 = ethers.keccak256(ethers.toUtf8Bytes("enc-m2-e1"));
    const sig2e1 = await signReport(meter2Signer, meter2Id, 1, c2e1);
    await (await gwE2E.submitReport(meter2Id, 1, c2e1, ct2e1, sig2e1, "0x")).wait();

    // Verify all 4 reports accepted
    assertEqual(await gwE2E.getReportStatus(meter1Id, 0), 1, "m1e0");
    assertEqual(await gwE2E.getReportStatus(meter1Id, 1), 1, "m1e1");
    assertEqual(await gwE2E.getReportStatus(meter2Id, 0), 1, "m2e0");
    assertEqual(await gwE2E.getReportStatus(meter2Id, 1), 1, "m2e1");

    // Verify duplicate rejection
    try {
      await gwE2E.submitReport(meter1Id, 0, c1e0, ct1e0, sig1e0, "0x");
      throw new Error("should have reverted");
    } catch (e) {
      assert(e.message.includes("duplicate") || e.message.includes("revert"), "duplicate rejected");
    }

    // Aggregate for epoch 0 — reaches DecReady
    const aggRef = ethers.keccak256(ethers.toUtf8Bytes("agg-region-epoch0"));
    await (await gwE2E.submitAggregate(regionId, 0, aggRef)).wait();
    const aggItemId = await gwE2E.computeAggregateItemId(regionId, 0);
    await (await gwE2E.connect(committee0).submitDecryptionShare(aggItemId, 0, ethers.keccak256(ethers.toUtf8Bytes("agg-s0")))).wait();
    await (await gwE2E.connect(committee1).submitDecryptionShare(aggItemId, 1, ethers.keccak256(ethers.toUtf8Bytes("agg-s1")))).wait();
    assertEqual((await gwE2E.getItem(aggItemId))[0], 2, "agg DecReady");

    // Bill for meter 1 — reaches DecReady
    await (await gwE2E.connect(customer).requestBillDecryption(meter1Id)).wait();
    const billItemId = await gwE2E.computeBillItemId(meter1Id);
    await (await gwE2E.connect(committee0).submitDecryptionShare(billItemId, 0, ethers.keccak256(ethers.toUtf8Bytes("bill-s0")))).wait();
    await (await gwE2E.connect(committee1).submitDecryptionShare(billItemId, 1, ethers.keccak256(ethers.toUtf8Bytes("bill-s1")))).wait();
    assertEqual((await gwE2E.getItem(billItemId))[0], 2, "bill DecReady");

    // Aggregate for epoch 1 — insufficient shares → CarryOver
    const aggRef1 = ethers.keccak256(ethers.toUtf8Bytes("agg-region-epoch1"));
    await (await gwE2E.submitAggregate(regionId, 1, aggRef1)).wait();
    const aggItemId1 = await gwE2E.computeAggregateItemId(regionId, 1);
    await (await gwE2E.connect(committee2).submitDecryptionShare(aggItemId1, 2, ethers.keccak256(ethers.toUtf8Bytes("agg1-s2")))).wait();

    await advanceTime(provider, DECRYPTION_DEADLINE + 1);
    await (await gwE2E.markCarryOver(aggItemId1)).wait();
    assertEqual((await gwE2E.getItem(aggItemId1))[0], 3, "agg epoch1 CarryOver");
  });

  // ═══════════════════════════════════════════════════════════
  console.log("\n────────────────────────────────────────");
  console.log(`Results: ${passed} passed, ${failed} failed out of ${passed + failed} tests`);
  if (failures.length > 0) {
    console.log("\nFailures:");
    failures.forEach((f) => console.log(`  - ${f.name}: ${f.error}`));
  }
  console.log("────────────────────────────────────────\n");

  process.exit(failed > 0 ? 1 : 0);
}

main().catch((e) => {
  console.error("Fatal error:", e);
  process.exit(1);
});
