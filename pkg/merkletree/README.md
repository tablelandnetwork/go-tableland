# Merkle Tree

This package implements a Merkle Tree. It is used to calculate the Merkle Root of a table and get Membership Proof of rows.

## Design characteristics

- It was designed to work with [OpenZepellin MerkeProof verifier](https://github.com/OpenZeppelin/openzeppelin-contracts/blob/260e082ed10e86e5870c4e5859750a8271eeb2b9/contracts/utils/cryptography/MerkleProof.sol#L27-L29). That requires leaves and hash pairs to be sorted. The sorting simplifies the proof verification by removing the need to include information about the order of the proof piece.
- It duplicates the last leaf node in case the number of leaves is odd. This is a common approach but vunerable to a forgery attack because two trees can produce the same Merkle Root, e.g. `MerkleRoot(a, b, c) = MerkleRoot(a, b, c, c)`. Not sure that will be a problem in our usecase.

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
fmt.Printf("ROOT: %s\n\n", hex.EncodeToString(tree.MerkleRoot()))

proof := tree.GetProof(crypto.Keccak256([]byte("D")))
for i, part := range proof {
    fmt.Printf("PROOF (%d): 0x%s\n", i, hex.EncodeToString(part))
}
```

```bash
ROOT: 8550ee72696375131758acb925f2aac02f94a06c7f796d9560df9aeb72f222d1

PROOF (0): 0x017e667f4b8c174291d1543c466717566e206df1bfd6f30271055ddafdb18f72
PROOF (1): 0x69de756fea16daddbbdccf85c849315f51c0b50d111e3d2063cab451803324a0
PROOF (2): 0x7f61b8bf6780bd017acc22aebf6e14d93aaca4d8b7d0b8fdbb22de1d8cc08126
```

## Things to explore

- Multi proofs
- Non-membership proofs
- Consistency proofs
- Serialization/Deserialization of the entire tree
- Possible Vunerabilites
- Performance
