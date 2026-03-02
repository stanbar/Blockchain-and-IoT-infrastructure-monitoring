// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * SmartGridGW — Verifiable Privacy-Preserving IoT Monitoring & Settlement
 *
 * Implements the SmartGrid-BC gateway state machine from the dissertation:
 *   Stage 0: Parameter pinning (deploy)
 *   Stage 1: Per-epoch private reporting (sg-report)
 *   Stage 2: Aggregate ciphertext path (sg-aggregate + sg-decshare for aggregates)
 *   Stage 3: Bill decryption path (sg-bill + sg-decshare for bills)
 *   Stage 4: Terminality / carry-over (sg-carryover)
 *
 * Crypto that would require heavy ZK or threshold ceremonies uses mock adapters
 * behind stable interfaces marked with TODO comments.
 */
contract SmartGridGW {
    // ----------------------------------------------------------------
    // Stage 0 — Parameter Pinning (alg:sg-deploy)
    // ----------------------------------------------------------------

    address public admin; // regulator / deployer

    /// Meter enrollment: Merkle root of enrolled meter public keys
    bytes32 public meterRoot;

    /// Threshold committee descriptor (n, t, public key commitment)
    uint8 public committeeN;
    uint8 public committeeT;
    bytes32 public committeeKeyCommitment;
    mapping(uint8 => address) public committeeMembers; // index => address

    /// Tariff coefficients pinned at deploy (simplified: single rate in wei-per-unit)
    uint256 public tariffRate;

    /// Schedule parameters
    uint256 public epochDuration;      // seconds per epoch
    uint256 public reportDeadline;     // seconds after epoch end to accept reports
    uint256 public decryptionDeadline; // seconds after aggregate/bill creation for shares
    uint256 public deployTimestamp;

    // ----------------------------------------------------------------
    // Stage 1 — Per-Epoch Private Reporting (alg:sg-report)
    // ----------------------------------------------------------------

    enum ReportStatus { None, Accepted }

    struct MeterReport {
        ReportStatus status;
        bytes32 commitment;      // commitment to the meter reading
        bytes32 ciphertextTag;   // tag identifying the encrypted reading
        bytes   signature;       // meter's signature over (meterId, epoch, commitment)
        // proof field omitted — verified on submit via adapter
    }

    /// reports[meterId][epoch] => MeterReport  (uniqueness: one per meter per epoch)
    mapping(bytes32 => mapping(uint256 => MeterReport)) public reports;

    /// Incremental bill ciphertext state per meter
    /// billCipherState[meterId] accumulates across epochs (XOR of ciphertext tags as placeholder)
    mapping(bytes32 => bytes32) public billCipherState;

    /// Track which epochs have been reported per meter
    mapping(bytes32 => uint256) public meterLastEpoch;

    // ----------------------------------------------------------------
    // Stage 2 & 3 — Aggregate / Bill Items
    // ----------------------------------------------------------------

    enum ItemStatus { None, Pending, DecReady, CarryOver }

    struct DecryptionItem {
        ItemStatus status;
        bytes32 ciphertextRef;        // reference to aggregate or bill ciphertext
        uint256 createdAt;            // block.timestamp when item was created
        uint8 sharesReceived;         // count of valid decryption shares
        mapping(uint8 => bytes32) shares; // committeeIndex => share
        mapping(uint8 => bool) hasShare;
    }

    /// items[itemId] — itemId is keccak256(type, regionOrMeter, epoch)
    mapping(bytes32 => DecryptionItem) internal _items;

    // Public view of item status (mappings inside structs can't be returned directly)
    function getItem(bytes32 itemId) external view returns (
        ItemStatus status,
        bytes32 ciphertextRef,
        uint256 createdAt,
        uint8 sharesReceived
    ) {
        DecryptionItem storage item = _items[itemId];
        return (item.status, item.ciphertextRef, item.createdAt, item.sharesReceived);
    }

    function getItemShare(bytes32 itemId, uint8 memberIndex) external view returns (bytes32) {
        return _items[itemId].shares[memberIndex];
    }

    // ----------------------------------------------------------------
    // Events — stable event model for off-chain indexing
    // ----------------------------------------------------------------

    event MeterEnrolled(bytes32 indexed meterId, bytes32 newMeterRoot);
    event ReportAccepted(bytes32 indexed meterId, uint256 indexed epoch, bytes32 commitment, bytes32 ciphertextTag);
    event AggregateCreated(bytes32 indexed itemId, bytes32 indexed regionId, uint256 indexed epoch, bytes32 ciphertextRef);
    event BillItemCreated(bytes32 indexed itemId, bytes32 indexed meterId, bytes32 ciphertextRef);
    event DecShareSubmitted(bytes32 indexed itemId, uint8 indexed memberIndex, bytes32 share);
    event ItemDecReady(bytes32 indexed itemId);
    event ItemCarryOver(bytes32 indexed itemId);
    event ParametersUpdated(uint256 tariffRate, uint256 epochDuration);

    // ----------------------------------------------------------------
    // Modifiers
    // ----------------------------------------------------------------

    modifier onlyAdmin() {
        require(msg.sender == admin, "only admin");
        _;
    }

    modifier onlyCommitteeMember(uint8 memberIndex) {
        require(committeeMembers[memberIndex] == msg.sender, "not committee member");
        _;
    }

    // ----------------------------------------------------------------
    // Constructor — alg:sg-deploy
    // ----------------------------------------------------------------

    constructor(
        bytes32 _meterRoot,
        uint8 _committeeN,
        uint8 _committeeT,
        bytes32 _committeeKeyCommitment,
        address[] memory _committeeAddresses,
        uint256 _tariffRate,
        uint256 _epochDuration,
        uint256 _reportDeadline,
        uint256 _decryptionDeadline
    ) {
        require(_committeeT > 0 && _committeeT <= _committeeN, "invalid threshold");
        require(_committeeAddresses.length == _committeeN, "committee size mismatch");
        require(_epochDuration > 0, "epoch duration must be > 0");

        admin = msg.sender;
        meterRoot = _meterRoot;
        committeeN = _committeeN;
        committeeT = _committeeT;
        committeeKeyCommitment = _committeeKeyCommitment;
        tariffRate = _tariffRate;
        epochDuration = _epochDuration;
        reportDeadline = _reportDeadline;
        decryptionDeadline = _decryptionDeadline;
        deployTimestamp = block.timestamp;

        for (uint8 i = 0; i < _committeeN; i++) {
            committeeMembers[i] = _committeeAddresses[i];
        }
    }

    // ----------------------------------------------------------------
    // Admin functions
    // ----------------------------------------------------------------

    function updateMeterRoot(bytes32 _newRoot) external onlyAdmin {
        meterRoot = _newRoot;
        emit MeterEnrolled(bytes32(0), _newRoot);
    }

    function updateParameters(uint256 _tariffRate, uint256 _epochDuration) external onlyAdmin {
        tariffRate = _tariffRate;
        epochDuration = _epochDuration;
        emit ParametersUpdated(_tariffRate, _epochDuration);
    }

    // ----------------------------------------------------------------
    // Stage 1 — submitReport (alg:sg-report)
    // ----------------------------------------------------------------

    /**
     * @notice Submit a private meter report for a given epoch.
     * @dev One accepted report per (meterId, epoch). Verifies signature and proof via adapter.
     * @param meterId     Identifier of the meter (hash of public key)
     * @param epoch       Epoch number
     * @param commitment  Commitment to the meter reading (e.g., Pedersen or hash commitment)
     * @param ciphertextTag  Tag/reference for the encrypted reading
     * @param signature   Meter's ECDSA signature over (meterId, epoch, commitment)
     * @param proof       ZK proof that commitment is well-formed (mock: ignored)
     */
    function submitReport(
        bytes32 meterId,
        uint256 epoch,
        bytes32 commitment,
        bytes32 ciphertextTag,
        bytes memory signature,
        bytes memory proof
    ) external {
        // Uniqueness: one report per (meter, epoch)
        require(reports[meterId][epoch].status == ReportStatus.None, "duplicate report");

        // Epoch validity check
        uint256 epochStart = deployTimestamp + epoch * epochDuration;
        require(block.timestamp >= epochStart, "epoch not started");
        require(block.timestamp <= epochStart + epochDuration + reportDeadline, "report deadline passed");

        // TODO: Verify meter is enrolled (Merkle proof against meterRoot)
        // For now, accept any meterId — mock adapter

        // Verify signature: recover signer from (meterId, epoch, commitment)
        bytes32 msgHash = keccak256(abi.encodePacked(meterId, epoch, commitment));
        bytes32 ethHash = _toEthSignedMessageHash(msgHash);
        address signer = _recoverSigner(ethHash, signature);
        require(signer != address(0), "invalid signature");

        // TODO: Verify ZK proof that commitment is well-formed
        // Mock adapter: accept any proof
        require(proof.length >= 0, "proof validation placeholder");

        // Store report
        reports[meterId][epoch] = MeterReport({
            status: ReportStatus.Accepted,
            commitment: commitment,
            ciphertextTag: ciphertextTag,
            signature: signature
        });

        // Incrementally update bill ciphertext state (XOR as placeholder for homomorphic accumulation)
        billCipherState[meterId] = billCipherState[meterId] ^ ciphertextTag;

        // Track last epoch
        if (epoch > meterLastEpoch[meterId]) {
            meterLastEpoch[meterId] = epoch;
        }

        emit ReportAccepted(meterId, epoch, commitment, ciphertextTag);
    }

    // ----------------------------------------------------------------
    // Stage 2 — submitAggregate (alg:sg-aggregate)
    // ----------------------------------------------------------------

    /**
     * @notice Register an aggregate ciphertext reference for a region/epoch.
     * @dev Creates a DecryptionItem in Pending status.
     * @param regionId       Region identifier
     * @param epoch          Epoch number
     * @param ciphertextRef  Reference to the aggregate ciphertext (off-chain)
     */
    function submitAggregate(
        bytes32 regionId,
        uint256 epoch,
        bytes32 ciphertextRef
    ) external onlyAdmin {
        bytes32 itemId = keccak256(abi.encodePacked("agg", regionId, epoch));
        require(_items[itemId].status == ItemStatus.None, "aggregate already exists");

        DecryptionItem storage item = _items[itemId];
        item.status = ItemStatus.Pending;
        item.ciphertextRef = ciphertextRef;
        item.createdAt = block.timestamp;
        item.sharesReceived = 0;

        emit AggregateCreated(itemId, regionId, epoch, ciphertextRef);
    }

    // ----------------------------------------------------------------
    // Stage 3 — requestBillDecryption (alg:sg-bill)
    // ----------------------------------------------------------------

    /**
     * @notice Register a bill decryption request for a meter.
     * @dev Creates a DecryptionItem in Pending status for the meter's accumulated bill ciphertext.
     * @param meterId  Meter identifier
     */
    function requestBillDecryption(bytes32 meterId) external {
        bytes32 ciphertextRef = billCipherState[meterId];
        require(ciphertextRef != bytes32(0), "no bill data for meter");

        bytes32 itemId = keccak256(abi.encodePacked("bill", meterId));
        require(_items[itemId].status == ItemStatus.None, "bill request already exists");

        DecryptionItem storage item = _items[itemId];
        item.status = ItemStatus.Pending;
        item.ciphertextRef = ciphertextRef;
        item.createdAt = block.timestamp;
        item.sharesReceived = 0;

        emit BillItemCreated(itemId, meterId, ciphertextRef);
    }

    // ----------------------------------------------------------------
    // Stages 2 & 3 — submitDecryptionShare (alg:sg-decshare)
    // ----------------------------------------------------------------

    /**
     * @notice Submit a threshold decryption share for any pending item.
     * @param itemId       The item being decrypted (aggregate or bill)
     * @param memberIndex  Committee member index
     * @param share        The decryption share (mock: any bytes32)
     */
    function submitDecryptionShare(
        bytes32 itemId,
        uint8 memberIndex,
        bytes32 share
    ) external onlyCommitteeMember(memberIndex) {
        DecryptionItem storage item = _items[itemId];
        require(item.status == ItemStatus.Pending, "item not pending");
        require(!item.hasShare[memberIndex], "share already submitted");
        require(block.timestamp <= item.createdAt + decryptionDeadline, "decryption deadline passed");

        // TODO: Verify decryption share correctness (DLEQ proof etc.)
        // Mock adapter: accept any share

        item.shares[memberIndex] = share;
        item.hasShare[memberIndex] = true;
        item.sharesReceived += 1;

        emit DecShareSubmitted(itemId, memberIndex, share);

        // Check if threshold reached
        if (item.sharesReceived >= committeeT) {
            item.status = ItemStatus.DecReady;
            emit ItemDecReady(itemId);
        }
    }

    // ----------------------------------------------------------------
    // Stage 4 — markCarryOver (alg:sg-carryover)
    // ----------------------------------------------------------------

    /**
     * @notice Mark a pending item as CarryOver if its decryption deadline has passed
     *         without reaching the threshold.
     * @param itemId  The item to mark as carried over
     */
    function markCarryOver(bytes32 itemId) external {
        DecryptionItem storage item = _items[itemId];
        require(item.status == ItemStatus.Pending, "item not pending");
        require(block.timestamp > item.createdAt + decryptionDeadline, "deadline not passed");

        item.status = ItemStatus.CarryOver;
        emit ItemCarryOver(itemId);
    }

    // ----------------------------------------------------------------
    // View helpers
    // ----------------------------------------------------------------

    function currentEpoch() public view returns (uint256) {
        if (block.timestamp < deployTimestamp) return 0;
        return (block.timestamp - deployTimestamp) / epochDuration;
    }

    function epochStartTime(uint256 epoch) public view returns (uint256) {
        return deployTimestamp + epoch * epochDuration;
    }

    function getReportStatus(bytes32 meterId, uint256 epoch) external view returns (ReportStatus) {
        return reports[meterId][epoch].status;
    }

    function getReportDetails(bytes32 meterId, uint256 epoch) external view returns (
        ReportStatus status,
        bytes32 commitment,
        bytes32 ciphertextTag
    ) {
        MeterReport storage r = reports[meterId][epoch];
        return (r.status, r.commitment, r.ciphertextTag);
    }

    function computeAggregateItemId(bytes32 regionId, uint256 epoch) external pure returns (bytes32) {
        return keccak256(abi.encodePacked("agg", regionId, epoch));
    }

    function computeBillItemId(bytes32 meterId) external pure returns (bytes32) {
        return keccak256(abi.encodePacked("bill", meterId));
    }

    // ----------------------------------------------------------------
    // Internal helpers
    // ----------------------------------------------------------------

    function _toEthSignedMessageHash(bytes32 hash) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", hash));
    }

    function _recoverSigner(bytes32 ethHash, bytes memory sig) internal pure returns (address) {
        if (sig.length != 65) return address(0);
        bytes32 r;
        bytes32 s;
        uint8 v;
        assembly {
            r := mload(add(sig, 32))
            s := mload(add(sig, 64))
            v := byte(0, mload(add(sig, 96)))
        }
        if (v < 27) v += 27;
        if (v != 27 && v != 28) return address(0);
        return ecrecover(ethHash, v, r, s);
    }
}
