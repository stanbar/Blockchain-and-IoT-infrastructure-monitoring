/**
 * Compile SmartGridGW.sol using local solcjs (no network needed).
 * Outputs artifact to artifacts/SmartGridGW.json
 */
const solc = require("solc");
const fs = require("fs");
const path = require("path");

const contractPath = path.join(__dirname, "..", "contracts", "SmartGridGW.sol");
const artifactsDir = path.join(__dirname, "..", "artifacts");
const source = fs.readFileSync(contractPath, "utf8");

const input = JSON.stringify({
  language: "Solidity",
  sources: { "SmartGridGW.sol": { content: source } },
  settings: {
    optimizer: { enabled: true, runs: 200 },
    outputSelection: { "*": { "*": ["abi", "evm.bytecode"] } },
  },
});

const output = JSON.parse(solc.compile(input));

if (output.errors) {
  const errors = output.errors.filter((e) => e.severity === "error");
  if (errors.length > 0) {
    errors.forEach((e) => console.error(e.formattedMessage));
    process.exit(1);
  }
  // Print warnings
  output.errors
    .filter((e) => e.severity === "warning")
    .forEach((e) => console.warn(e.formattedMessage));
}

const contract = output.contracts["SmartGridGW.sol"]["SmartGridGW"];
const artifact = {
  contractName: "SmartGridGW",
  abi: contract.abi,
  bytecode: "0x" + contract.evm.bytecode.object,
};

fs.mkdirSync(artifactsDir, { recursive: true });
fs.writeFileSync(
  path.join(artifactsDir, "SmartGridGW.json"),
  JSON.stringify(artifact, null, 2)
);

console.log(
  `Compiled SmartGridGW.sol -> artifacts/SmartGridGW.json (ABI: ${contract.abi.length} entries, bytecode: ${artifact.bytecode.length} chars)`
);
