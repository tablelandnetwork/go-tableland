# Merkle Tree

This package implements a Merkle Tree. It is used to calculate the Merkle Root of a table and get Membership Proof of rows.

## Design characteristics

- It was designed to work with [OpenZepellin MerkeProof verifier](https://github.com/OpenZeppelin/openzeppelin-contracts/blob/260e082ed10e86e5870c4e5859750a8271eeb2b9/contracts/utils/cryptography/MerkleProof.sol#L27-L29). That requires leaves and hash pairs to be sorted. The sorting simplifies the proof verification by removing the need to include information about the order of the proof piece.
- It duplicates the last leaf node in case the number of leaves is odd. This is a common approach but vunerable to a forgery attack because two trees can produce the same Merkle Root, e.g. `MerkleRoot(a, b, c) = MerkleRoot(a, b, c, c)`. But that is not a problem in our use case.

## Usage

```go
    leaves := [][]byte{}
    leaves = append(leaves, []byte("A"))
    leaves = append(leaves, []byte("B"))
    leaves = append(leaves, []byte("C"))
    leaves = append(leaves, []byte("D"))
    leaves = append(leaves, []byte("E"))
    leaves = append(leaves, []byte("F"))
    tree, _ := merkletree.NewTree(leaves, crypto.Keccak256)

    // Getting the root
    root := tree.MerkleRoot()
    fmt.Printf("ROOT: %s\n\n", hex.EncodeToString(root))

    // Getting the proof for a given leaf
    leaf := crypto.Keccak256([]byte("D"))
    proof := tree.GetProof(leaf)
    for i, part := range proof {
        fmt.Printf("PROOF (%d): 0x%s\n", i, hex.EncodeToString(part))
    }

    // Verifying the proof
    ok := merkletree.VerifyProof(root, proof, leaf, crypto.Keccak256)
    fmt.Printf("\nVERIFICATION RESULT: %t\n", ok)
```

```bash
ROOT: 5b4f920caf9a50816be944fd3626945ebaed5fcd1f041fa864027d4eaad29cf6

PROOF (0): 0xe61d9a3d3848fb2cdd9a2ab61e2f21a10ea431275aed628a0557f9dee697c37a
PROOF (1): 0x324d51074ba12c3b56f59e6a9dd606351316426b7f7d924b1fc9efa7f261b476
PROOF (2): 0xd8c26fda8cf7503459d00730efe60ff9ec19bf97b7a26b6aa42fa8d8337efe78

VERIFICATION RESULT: true
```

## Things to explore

- Multi proofs
- Non-membership proofs
- Consistency proofs
- Serialization/Deserialization of the entire tree
- Possible Vunerabilites
- Performance
