# Privacy-Preserving, Scalable Blockchain-Based Solution for Monitoring Industrial Infrastructure in the Near Real-Time

## Authors

### Stanislaw Baranski

Department of Electronic, Telecommunication and Informatics, Gdansk University of Technology

stanislaw.baranski@pg.edu.pl

### Andrzej Sobecki

Department of Electronic, Telecommunication and Informatics, Gdansk University of Technology

andrzej.sobecki@pg.edu.pl

### Julian Szymanski

Department of Electronic, Telecommunication and Informatics, Gdansk University of Technology

julian.szymanski@eti.pg.edu.pl

## Abstract

This paper proposes an improved monitoring and measuring system dedicated to industrial infrastructure. Our model achieves security of data by incorporating cryptographical methods and near real-time access by the use of virtual tree structure over records. The currently available blockchain networks are not very well adapted to tasks related to the continuous monitoring of the parameters of industrial installations. In the database systems delivered by default (the so-called world state), only the resultant or the last value recorded by the IoT device is stored. Effective use of measurement values recorded in the past requires each time viewing the entire chain of recorded events for a given IoT device. The solution proposed in the article introduces the concept of dependent wallets, the purpose of which is the aggregation and indexation of changes in machine parameters, recorded in the original wallets. As a result, we can easily get data from a certain sensor or sensors in the specified date range, even if the chain of transactions is very long. Our contribution is a universal mechanism that improves the efficiency of the infrastructure monitoring process, which uses blockchains to record measurements from sensors. The proposed model has been experimentally tested on two types of blockchains: Stellar and Hyperledger Fabric.

**Keywords**: blockchain; Hyperledger Fabric; Stellar; IoT


[Read more](https://www.mdpi.com/2076-3417/12/14/7143/htm)

---

# SmartGrid-BC: Verifiable Privacy-Preserving IoT Monitoring and Settlement (PoC)

A minimal but working proof-of-concept implementing the SmartGrid-BC protocol from the dissertation. Demonstrates private per-epoch meter reporting, on-chain report acceptance with uniqueness, aggregate/bill ciphertext handling, threshold decryption-share collection, and explicit carry-over fallback — all on an EVM chain.

## Quick Start

### Prerequisites

- Node.js >= 18

### 1. Install dependencies

```bash
cd smartgrid && npm install
cd webapp && npm install
cd ../..
```

### 2. Compile the contract

```bash
cd smartgrid
node scripts/compile.js
```

### 3. Run tests (26 tests)

```bash
# Terminal 1: start local EVM node
cd smartgrid && npx hardhat node

# Terminal 2: run tests
cd smartgrid && node test/run-tests.js
```

### 4. Run the CLI demo

```bash
# (with hardhat node running)
cd smartgrid && node scripts/demo.js
```

### 5. Run the web UI

```bash
# Terminal 1: local EVM node
cd smartgrid && npx hardhat node

# Terminal 2: web app
cd smartgrid/webapp && npm run dev
# Open http://localhost:3000
```

In the web UI:
1. Click **Deploy Contract** to deploy SmartGridGW
2. Use **Meter Simulator** buttons to submit reports
3. Click **Advance 1 Epoch** to move to the next epoch
4. Use **Submit Aggregate** to create aggregate items
5. Use **Committee** share buttons to submit decryption shares (2-of-3 threshold)
6. Use **Customer** panel to request bill decryption
7. Use **Advance Past Deadline** + **CarryOver** to test timeout fallback

## Implemented Features

| Feature | Status | Dissertation Reference |
|---|---|---|
| Parameter pinning (meter root, committee, tariff, schedule) | Done | alg:sg-deploy |
| Per-epoch private reporting with (meter, epoch) uniqueness | Done | alg:sg-report |
| ECDSA signature verification on reports | Done | alg:sg-report |
| Incremental bill ciphertext state (XOR accumulation) | Done | alg:sg-report |
| Aggregate ciphertext registration | Done | alg:sg-aggregate |
| Threshold decryption share collection | Done | alg:sg-decshare |
| DecReady state transition when threshold met | Done | alg:sg-decshare |
| Bill decryption request + threshold shares | Done | alg:sg-bill |
| CarryOver fallback on deadline expiry | Done | alg:sg-carryover |
| Web UI with 4 roles (admin, meter, committee, customer) | Done | — |
| End-to-end demo: 2 meters, 2 epochs | Done | — |

## Mocked Components

| Component | What's mocked | How to replace |
|---|---|---|
| **ZK Proof Verification** | `proof` parameter accepted but not verified (any bytes pass) | Implement `IProofVerifier` interface; replace the `require(proof.length >= 0)` in `submitReport()` with a call to a real Groth16/PLONK verifier contract (e.g., snarkjs + solidity verifier) |
| **Meter Enrollment** | Any `meterId` accepted without Merkle proof against `meterRoot` | Add a `bytes32[] merkleProof` parameter to `submitReport()` and verify `MerkleProof.verify(proof, meterRoot, leaf)` using OpenZeppelin's `MerkleProof` library |
| **Commitment Scheme** | `keccak256(reading)` used as commitment | Replace with Pedersen commitment `g^v * h^r` for hiding+binding, or use a circuit-friendly hash |
| **Ciphertext Model** | `bytes32` tags used as ciphertext references; XOR for accumulation | Replace with ElGamal or threshold-BFV ciphertext structs; accumulation becomes homomorphic addition |
| **Decryption Share Verification** | Any `bytes32` accepted as a valid share | Implement DLEQ proof verification per share to ensure correctness against the committee public key |
| **Threshold Key Setup** | Committee addresses set at deploy; no DKG ceremony | Implement FDKG protocol (sec:fdkg-full) to establish committee key shares distributedly |
| **Off-chain Reconstruction** | Decrypted plaintexts not reconstructed from shares | Implement Lagrange interpolation over decryption shares to recover plaintext aggregate/bill values |

## Architecture

```
smartgrid/
├── contracts/
│   └── SmartGridGW.sol          # Core gateway contract (all 5 stages)
├── scripts/
│   ├── compile.js               # Compile with solcjs (no network needed)
│   ├── deploy.js                # Deploy to local node
│   └── demo.js                  # Full E2E demo script
├── test/
│   ├── run-tests.js             # 26 tests (ethers.js + hardhat node)
│   └── SmartGridGW.test.ts      # Hardhat test (for when solc download works)
├── webapp/
│   ├── app/
│   │   ├── page.tsx             # Main UI with all 4 role panels
│   │   ├── layout.tsx           # Root layout
│   │   └── api/deploy/route.ts  # Server-side deploy endpoint
│   └── lib/
│       └── contract.ts          # ABI, helpers, constants
├── hardhat.config.ts
└── package.json
```

## Known Risks

1. **No real privacy**: Readings are committed with keccak256 (not hiding); ciphertexts are placeholder tags. An adversary with the plaintext can verify a commitment.
2. **No real threshold cryptography**: Decryption shares are not verified. Any committee member can submit arbitrary bytes32 values.
3. **Epoch timing is block.timestamp-based**: Miners/validators can manipulate timestamps within limits. For a PoC this is acceptable.
4. **Single contract**: All state lives in one contract. Production would split into registry, reporting, and settlement contracts.
5. **Hardhat test accounts**: The webapp uses hardhat's default private keys. Never use in production.

## Next 3 Concrete Tasks

1. **Replace mock proof verifier**: Integrate a Groth16 verifier (e.g., from circom/snarkjs) that checks the report commitment is well-formed with respect to a hidden reading and randomness.
2. **Add Merkle-based meter enrollment**: Use OpenZeppelin `MerkleProof` to verify meters are enrolled before accepting reports. Build a simple enrollment script that generates the tree.
3. **Implement real ciphertext accumulation**: Replace XOR placeholders with additively-homomorphic ElGamal over a suitable curve, so bill ciphertext state is a meaningful encryption of cumulative usage.

## Test Results

```
26 passed, 0 failed out of 26 tests

Stage 0: Parameter Pinning           — 6 tests
Stage 1: Per-Epoch Private Reporting — 6 tests
Stage 2: Aggregate Ciphertext Path   — 6 tests
Stage 3: Bill Decryption Path        — 3 tests
Stage 4: Terminality / Carry-Over    — 4 tests
End-to-End: 2 Meters, 2 Epochs      — 1 test
```
