"use client";

import { useState, useEffect, useCallback } from "react";
import {
  createPublicClient,
  createWalletClient,
  http,
  keccak256,
  toBytes,
  toHex,
  encodePacked,
  type Address,
  type Hex,
} from "viem";
import { privateKeyToAccount } from "viem/accounts";
import { hardhat } from "viem/chains";

// ─── Contract ABI (subset matching SmartGridGW.sol) ───
const abi = [
  { type: "function", name: "admin", inputs: [], outputs: [{ type: "address" }], stateMutability: "view" },
  { type: "function", name: "committeeN", inputs: [], outputs: [{ type: "uint8" }], stateMutability: "view" },
  { type: "function", name: "committeeT", inputs: [], outputs: [{ type: "uint8" }], stateMutability: "view" },
  { type: "function", name: "epochDuration", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "reportDeadline", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "decryptionDeadline", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "deployTimestamp", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "currentEpoch", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "tariffRate", inputs: [], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "billCipherState", inputs: [{ type: "bytes32" }], outputs: [{ type: "bytes32" }], stateMutability: "view" },
  { type: "function", name: "meterLastEpoch", inputs: [{ type: "bytes32" }], outputs: [{ type: "uint256" }], stateMutability: "view" },
  { type: "function", name: "getReportStatus", inputs: [{ type: "bytes32" }, { type: "uint256" }], outputs: [{ type: "uint8" }], stateMutability: "view" },
  { type: "function", name: "getReportDetails", inputs: [{ type: "bytes32" }, { type: "uint256" }], outputs: [{ type: "uint8" }, { type: "bytes32" }, { type: "bytes32" }], stateMutability: "view" },
  { type: "function", name: "getItem", inputs: [{ type: "bytes32" }], outputs: [{ type: "uint8" }, { type: "bytes32" }, { type: "uint256" }, { type: "uint8" }], stateMutability: "view" },
  { type: "function", name: "computeAggregateItemId", inputs: [{ type: "bytes32" }, { type: "uint256" }], outputs: [{ type: "bytes32" }], stateMutability: "pure" },
  { type: "function", name: "computeBillItemId", inputs: [{ type: "bytes32" }], outputs: [{ type: "bytes32" }], stateMutability: "pure" },
  { type: "function", name: "submitReport", inputs: [{ type: "bytes32" }, { type: "uint256" }, { type: "bytes32" }, { type: "bytes32" }, { type: "bytes" }, { type: "bytes" }], outputs: [], stateMutability: "nonpayable" },
  { type: "function", name: "submitAggregate", inputs: [{ type: "bytes32" }, { type: "uint256" }, { type: "bytes32" }], outputs: [], stateMutability: "nonpayable" },
  { type: "function", name: "requestBillDecryption", inputs: [{ type: "bytes32" }], outputs: [], stateMutability: "nonpayable" },
  { type: "function", name: "submitDecryptionShare", inputs: [{ type: "bytes32" }, { type: "uint8" }, { type: "bytes32" }], outputs: [], stateMutability: "nonpayable" },
  { type: "function", name: "markCarryOver", inputs: [{ type: "bytes32" }], outputs: [], stateMutability: "nonpayable" },
  // Events
  { type: "event", name: "ReportAccepted", inputs: [{ type: "bytes32", indexed: true, name: "meterId" }, { type: "uint256", indexed: true, name: "epoch" }, { type: "bytes32", name: "commitment" }, { type: "bytes32", name: "ciphertextTag" }] },
  { type: "event", name: "AggregateCreated", inputs: [{ type: "bytes32", indexed: true, name: "itemId" }, { type: "bytes32", indexed: true, name: "regionId" }, { type: "uint256", indexed: true, name: "epoch" }, { type: "bytes32", name: "ciphertextRef" }] },
  { type: "event", name: "BillItemCreated", inputs: [{ type: "bytes32", indexed: true, name: "itemId" }, { type: "bytes32", indexed: true, name: "meterId" }, { type: "bytes32", name: "ciphertextRef" }] },
  { type: "event", name: "ItemDecReady", inputs: [{ type: "bytes32", indexed: true, name: "itemId" }] },
  { type: "event", name: "ItemCarryOver", inputs: [{ type: "bytes32", indexed: true, name: "itemId" }] },
] as const;

// Hardhat test accounts
const PRIVATE_KEYS = [
  "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
  "0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d",
  "0x5de4111afa1a4b94908f83103eb1f1706367c2e68ca870fc3fb9a804cdab365a",
  "0x7c852118294e51e653712a81e05800f419141751be58f605c371e15141b007a6",
  "0x47e179ec197488593b187f80a00eb0da91f1b9d0b13f8733639f19c30a34926a",
  "0x8b3a350cf5c34c9194ca85829a2df0ec3153be0318b5e2d3348e872092edffba",
  "0x92db14e403b83dfe3df233f83dfa3a0d7096f21ca9b0d6d6b8d88b2b4ec1564e",
] as const;

const ACCOUNTS = PRIVATE_KEYS.map((pk) => privateKeyToAccount(pk as Hex));
const ROLE_NAMES = ["Admin/Regulator", "Meter 1", "Meter 2", "Committee 0", "Committee 1", "Committee 2", "Customer"];
const REGION_ID = keccak256(toBytes("region-A"));

const ITEM_STATUSES = ["None", "Pending", "DecReady", "CarryOver"];
const REPORT_STATUSES = ["None", "Accepted"];

const publicClient = createPublicClient({ chain: hardhat, transport: http() });

function walletClient(index: number) {
  return createWalletClient({
    chain: hardhat,
    transport: http(),
    account: ACCOUNTS[index],
  });
}

function meterId(accountIndex: number): Hex {
  return keccak256(ACCOUNTS[accountIndex].address as Hex);
}

type LogEntry = { ts: string; msg: string; ok: boolean };

export default function Home() {
  const [contractAddr, setContractAddr] = useState("");
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [contractInfo, setContractInfo] = useState<Record<string, string>>({});
  const [reportResults, setReportResults] = useState<Record<string, string>>({});
  const [itemResults, setItemResults] = useState<Record<string, string>>({});

  const log = useCallback((msg: string, ok = true) => {
    setLogs((prev) => [{ ts: new Date().toLocaleTimeString(), msg, ok }, ...prev.slice(0, 99)]);
  }, []);

  const addr = contractAddr as Address;

  // ─── Fetch contract state ───
  const refreshState = useCallback(async () => {
    if (!contractAddr) return;
    try {
      const [epoch, epochDur, tariff, deplTs, repDl, decDl, commN, commT] = await Promise.all([
        publicClient.readContract({ address: addr, abi, functionName: "currentEpoch" }),
        publicClient.readContract({ address: addr, abi, functionName: "epochDuration" }),
        publicClient.readContract({ address: addr, abi, functionName: "tariffRate" }),
        publicClient.readContract({ address: addr, abi, functionName: "deployTimestamp" }),
        publicClient.readContract({ address: addr, abi, functionName: "reportDeadline" }),
        publicClient.readContract({ address: addr, abi, functionName: "decryptionDeadline" }),
        publicClient.readContract({ address: addr, abi, functionName: "committeeN" }),
        publicClient.readContract({ address: addr, abi, functionName: "committeeT" }),
      ]);
      setContractInfo({
        currentEpoch: String(epoch),
        epochDuration: `${epochDur}s`,
        tariffRate: `${tariff} wei/unit`,
        deployTimestamp: new Date(Number(deplTs) * 1000).toISOString(),
        reportDeadline: `${repDl}s`,
        decryptionDeadline: `${decDl}s`,
        committee: `${commT}-of-${commN}`,
      });
    } catch (e: any) {
      log(`Failed to read contract: ${e.message}`, false);
    }
  }, [contractAddr, addr, log]);

  useEffect(() => {
    if (contractAddr) refreshState();
  }, [contractAddr, refreshState]);

  // ─── Deploy ───
  async function handleDeploy() {
    try {
      log("Deploying SmartGridGW...");
      const res = await fetch("/api/deploy", { method: "POST" });
      const data = await res.json();
      if (data.error) throw new Error(data.error);
      setContractAddr(data.address);
      log(`Deployed at ${data.address}`);
      setTimeout(refreshState, 500);
    } catch (e: any) {
      log(`Deploy failed: ${e.message}`, false);
    }
  }

  // ─── Submit Report (Meter Simulator) ───
  async function handleSubmitReport(meterIndex: number) {
    if (!contractAddr) return log("Deploy first", false);
    try {
      const epoch = Number(contractInfo.currentEpoch || 0);
      const mid = meterId(meterIndex);
      const reading = Math.floor(Math.random() * 100) + 10;
      const commitment = keccak256(toBytes(`reading-${reading}kwh-${Date.now()}`));
      const ciphertextTag = keccak256(toBytes(`enc-${reading}-${Date.now()}`));

      // Sign: keccak256(abi.encodePacked(meterId, epoch, commitment))
      const msgHash = keccak256(encodePacked(["bytes32", "uint256", "bytes32"], [mid, BigInt(epoch), commitment]));
      const signature = await ACCOUNTS[meterIndex].signMessage({ message: { raw: toBytes(msgHash) } });

      const wc = walletClient(meterIndex);
      const hash = await wc.writeContract({
        address: addr, abi, functionName: "submitReport",
        args: [mid, BigInt(epoch), commitment, ciphertextTag, signature as Hex, "0x"],
      });
      await publicClient.waitForTransactionReceipt({ hash });
      log(`Meter ${meterIndex}: Report accepted for epoch ${epoch} (${reading} kWh)`);
      setReportResults((prev) => ({ ...prev, [`m${meterIndex}e${epoch}`]: `${REPORT_STATUSES[1]} (${reading} kWh)` }));
      refreshState();
    } catch (e: any) {
      const msg = e.message?.includes("duplicate") ? "Duplicate report rejected" : e.message?.slice(0, 100);
      log(`Meter ${meterIndex} report failed: ${msg}`, false);
    }
  }

  // ─── Submit Aggregate (Admin/Operator) ───
  async function handleSubmitAggregate() {
    if (!contractAddr) return log("Deploy first", false);
    try {
      const epoch = Number(contractInfo.currentEpoch || 0);
      const ciphertextRef = keccak256(toBytes(`aggregate-${REGION_ID}-e${epoch}-${Date.now()}`));
      const wc = walletClient(0); // admin
      const hash = await wc.writeContract({
        address: addr, abi, functionName: "submitAggregate",
        args: [REGION_ID, BigInt(epoch), ciphertextRef],
      });
      await publicClient.waitForTransactionReceipt({ hash });
      const itemId = await publicClient.readContract({ address: addr, abi, functionName: "computeAggregateItemId", args: [REGION_ID, BigInt(epoch)] });
      log(`Aggregate created for region epoch ${epoch}: ${(itemId as string).slice(0, 18)}...`);
      await refreshItemState(itemId as Hex, `agg-e${epoch}`);
      refreshState();
    } catch (e: any) {
      log(`Aggregate failed: ${e.message?.slice(0, 100)}`, false);
    }
  }

  // ─── Submit Decryption Share ───
  async function handleSubmitShare(committeeIndex: number, itemKey: string, itemId: Hex) {
    if (!contractAddr) return log("Deploy first", false);
    try {
      const share = keccak256(toBytes(`share-${committeeIndex}-${Date.now()}`));
      const wc = walletClient(3 + committeeIndex); // accounts 3,4,5 are committee
      const hash = await wc.writeContract({
        address: addr, abi, functionName: "submitDecryptionShare",
        args: [itemId, committeeIndex, share],
      });
      await publicClient.waitForTransactionReceipt({ hash });
      log(`Committee ${committeeIndex}: Share submitted for ${itemKey}`);
      await refreshItemState(itemId, itemKey);
    } catch (e: any) {
      log(`Share failed: ${e.message?.slice(0, 100)}`, false);
    }
  }

  // ─── Request Bill Decryption (Customer) ───
  async function handleRequestBill(meterIndex: number) {
    if (!contractAddr) return log("Deploy first", false);
    try {
      const mid = meterId(meterIndex);
      const wc = walletClient(6); // customer
      const hash = await wc.writeContract({
        address: addr, abi, functionName: "requestBillDecryption",
        args: [mid],
      });
      await publicClient.waitForTransactionReceipt({ hash });
      const billItemId = await publicClient.readContract({ address: addr, abi, functionName: "computeBillItemId", args: [mid] });
      log(`Bill decryption requested for meter ${meterIndex}`);
      await refreshItemState(billItemId as Hex, `bill-m${meterIndex}`);
    } catch (e: any) {
      log(`Bill request failed: ${e.message?.slice(0, 100)}`, false);
    }
  }

  // ─── Mark CarryOver ───
  async function handleCarryOver(itemKey: string, itemId: Hex) {
    if (!contractAddr) return log("Deploy first", false);
    try {
      const wc = walletClient(0);
      const hash = await wc.writeContract({
        address: addr, abi, functionName: "markCarryOver", args: [itemId],
      });
      await publicClient.waitForTransactionReceipt({ hash });
      log(`Item ${itemKey} marked as CarryOver`);
      await refreshItemState(itemId, itemKey);
    } catch (e: any) {
      log(`CarryOver failed: ${e.message?.slice(0, 100)}`, false);
    }
  }

  // ─── Advance epoch (mine blocks) ───
  async function handleAdvanceEpoch() {
    try {
      const epochDur = Number(String(contractInfo.epochDuration).replace("s", "")) || 60;
      await fetch("http://127.0.0.1:8545", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jsonrpc: "2.0", method: "evm_increaseTime", params: [epochDur], id: 1 }),
      });
      await fetch("http://127.0.0.1:8545", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jsonrpc: "2.0", method: "evm_mine", params: [], id: 2 }),
      });
      log(`Advanced time by ${epochDur}s (1 epoch)`);
      refreshState();
    } catch (e: any) {
      log(`Time advance failed: ${e.message}`, false);
    }
  }

  // ─── Advance past decryption deadline ───
  async function handleAdvanceDeadline() {
    try {
      const dl = Number(String(contractInfo.decryptionDeadline).replace("s", "")) || 300;
      await fetch("http://127.0.0.1:8545", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jsonrpc: "2.0", method: "evm_increaseTime", params: [dl + 1], id: 1 }),
      });
      await fetch("http://127.0.0.1:8545", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ jsonrpc: "2.0", method: "evm_mine", params: [], id: 2 }),
      });
      log(`Advanced time by ${dl + 1}s (past decryption deadline)`);
      refreshState();
    } catch (e: any) {
      log(`Time advance failed: ${e.message}`, false);
    }
  }

  // ─── Refresh item state ───
  async function refreshItemState(itemId: Hex, key: string) {
    try {
      const item = await publicClient.readContract({ address: addr, abi, functionName: "getItem", args: [itemId] });
      const [status, , , shares] = item as [number, Hex, bigint, number];
      setItemResults((prev) => ({
        ...prev,
        [key]: `${ITEM_STATUSES[status]} (${shares} shares) [${itemId.slice(0, 18)}...]`,
      }));
    } catch {}
  }

  // ─── Tracked items for the committee UI ───
  const trackedItems = Object.entries(itemResults).filter(([, v]) => v.includes("Pending"));

  // ─── Render ───
  return (
    <div style={{ maxWidth: 1200, margin: "0 auto", padding: 20 }}>
      <h1 style={{ borderBottom: "2px solid #333", paddingBottom: 10 }}>SmartGrid-BC Demo</h1>
      <p style={{ color: "#666", marginTop: 0 }}>
        Verifiable Privacy-Preserving IoT Monitoring and Settlement on EVM
      </p>

      {/* Deploy */}
      <Section title="Setup">
        <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
          <button onClick={handleDeploy} style={btnStyle("#2563eb")}>Deploy Contract</button>
          {contractAddr && (
            <span style={{ fontFamily: "monospace", fontSize: 13 }}>
              Contract: {contractAddr}
            </span>
          )}
        </div>
        {contractAddr && (
          <div style={{ marginTop: 10, display: "flex", gap: 10, flexWrap: "wrap" }}>
            {Object.entries(contractInfo).map(([k, v]) => (
              <Tag key={k} label={k} value={v} />
            ))}
          </div>
        )}
      </Section>

      {/* Admin / Operator */}
      <Section title="Admin / Regulator / Operator" color="#7c3aed">
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
          <button onClick={handleAdvanceEpoch} style={btnStyle("#7c3aed")}>Advance 1 Epoch</button>
          <button onClick={handleAdvanceDeadline} style={btnStyle("#7c3aed")}>Advance Past Deadline</button>
          <button onClick={handleSubmitAggregate} style={btnStyle("#7c3aed")}>Submit Aggregate (current epoch)</button>
          {trackedItems.map(([key, val]) => {
            const idMatch = val.match(/\[0x[a-f0-9]+/);
            const itemId = idMatch ? (idMatch[0].slice(1).replace("...", "") as Hex) : ("0x" as Hex);
            return (
              <button key={`co-${key}`} onClick={() => handleCarryOver(key, itemId)} style={btnStyle("#dc2626")}>
                CarryOver: {key}
              </button>
            );
          })}
        </div>
        <button onClick={refreshState} style={{ ...btnStyle("#6b7280"), marginTop: 8 }}>Refresh State</button>
      </Section>

      {/* Meter Simulator */}
      <Section title="Meter Simulator" color="#059669">
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
          <button onClick={() => handleSubmitReport(1)} style={btnStyle("#059669")}>
            Meter 1: Submit Report
          </button>
          <button onClick={() => handleSubmitReport(2)} style={btnStyle("#059669")}>
            Meter 2: Submit Report
          </button>
        </div>
        {Object.keys(reportResults).length > 0 && (
          <div style={{ marginTop: 10 }}>
            <strong>Reports:</strong>
            {Object.entries(reportResults).map(([k, v]) => (
              <Tag key={k} label={k} value={v} />
            ))}
          </div>
        )}
      </Section>

      {/* Committee / Relayer */}
      <Section title="Threshold Committee (Relayer)" color="#d97706">
        <p style={{ margin: "0 0 8px", fontSize: 13, color: "#666" }}>
          Submit decryption shares for pending items. Threshold: {contractInfo.committee || "?"}
        </p>
        {Object.entries(itemResults).map(([key, val]) => {
          const idMatch = val.match(/\[0x[a-f0-9]+/);
          const rawId = idMatch ? idMatch[0].slice(1).replace("...", "") : "";
          return (
            <div key={key} style={{ marginBottom: 8, padding: 8, background: "#fff8f0", borderRadius: 6 }}>
              <strong>{key}:</strong> {val}
              {val.includes("Pending") && (
                <div style={{ marginTop: 4, display: "flex", gap: 6 }}>
                  {[0, 1, 2].map((i) => (
                    <button
                      key={i}
                      onClick={() => handleSubmitShare(i, key, rawId as Hex)}
                      style={btnStyle("#d97706", true)}
                    >
                      Share from C{i}
                    </button>
                  ))}
                </div>
              )}
            </div>
          );
        })}
        {Object.keys(itemResults).length === 0 && (
          <p style={{ color: "#999", margin: 0 }}>No items yet. Submit an aggregate or bill request first.</p>
        )}
      </Section>

      {/* Customer View */}
      <Section title="Customer / User View" color="#0891b2">
        <div style={{ display: "flex", gap: 10, flexWrap: "wrap" }}>
          <button onClick={() => handleRequestBill(1)} style={btnStyle("#0891b2")}>
            Request Bill Decryption (Meter 1)
          </button>
          <button onClick={() => handleRequestBill(2)} style={btnStyle("#0891b2")}>
            Request Bill Decryption (Meter 2)
          </button>
        </div>
        {Object.entries(itemResults)
          .filter(([k]) => k.startsWith("bill"))
          .map(([k, v]) => (
            <Tag key={k} label={k} value={v} />
          ))}
      </Section>

      {/* Activity Log */}
      <Section title="Activity Log">
        <div style={{ maxHeight: 250, overflow: "auto", fontFamily: "monospace", fontSize: 12, lineHeight: 1.6 }}>
          {logs.map((l, i) => (
            <div key={i} style={{ color: l.ok ? "#16a34a" : "#dc2626" }}>
              <span style={{ color: "#999" }}>[{l.ts}]</span> {l.msg}
            </div>
          ))}
          {logs.length === 0 && <span style={{ color: "#999" }}>No activity yet. Deploy a contract to start.</span>}
        </div>
      </Section>
    </div>
  );
}

// ─── UI helpers ───

function Section({ title, color, children }: { title: string; color?: string; children: React.ReactNode }) {
  return (
    <div style={{ marginBottom: 20, padding: 16, background: "#fff", borderRadius: 8, border: `1px solid #e5e7eb`, borderLeft: `4px solid ${color || "#6b7280"}` }}>
      <h2 style={{ margin: "0 0 12px", fontSize: 16, color: color || "#333" }}>{title}</h2>
      {children}
    </div>
  );
}

function Tag({ label, value }: { label: string; value: string }) {
  return (
    <span style={{ display: "inline-block", margin: "2px 4px", padding: "2px 8px", background: "#f3f4f6", borderRadius: 4, fontSize: 12, fontFamily: "monospace" }}>
      <strong>{label}:</strong> {value}
    </span>
  );
}

function btnStyle(color: string, small = false): React.CSSProperties {
  return {
    background: color,
    color: "#fff",
    border: "none",
    borderRadius: 6,
    padding: small ? "4px 10px" : "8px 16px",
    fontSize: small ? 12 : 14,
    cursor: "pointer",
    fontWeight: 500,
  };
}
