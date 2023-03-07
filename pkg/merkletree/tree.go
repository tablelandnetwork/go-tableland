package merkletree

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"sort"

	"golang.org/x/crypto/sha3"
)

// DefaultHasher is the default hash function in case none is passed.
var DefaultHasher = sha3.NewLegacyKeccak256

// EncodingSchemaNLBytes indicates the number of bytes of the number of leaves in the encoding schema.
const EncodingSchemaNLBytes = 4

// MerkleTree is a binary Merkle Tree implemenation.
type MerkleTree struct {
	root   *Node
	leaves []*Node

	h hash.Hash
}

// Node represents a Node of MerkleTree.
type Node struct {
	parent, left, right *Node
	hash                []byte
}

// Proof represents a proof.
type Proof [][]byte

// Hex is a hex encoded representation of a proof.
func (p Proof) Hex() []string {
	pieces := make([]string, len(p))
	for i, part := range p {
		pieces[i] = hex.EncodeToString(part)
	}

	return pieces
}

func (n *Node) isLeaf() bool {
	return n.left == nil && n.right == nil
}

// NewTree builds a new Merkle Tree.
func NewTree(leaves [][]byte, hasher func() hash.Hash) (*MerkleTree, error) {
	if hasher == nil {
		hasher = DefaultHasher
	}

	tree := &MerkleTree{
		h: hasher(),
	}

	if len(leaves) == 0 {
		return nil, errors.New("no leaves")
	}

	if err := tree.buildTree(leaves); err != nil {
		return nil, fmt.Errorf("building the tree: %s", err)
	}
	return tree, nil
}

func (t *MerkleTree) buildTree(leaves [][]byte) error {
	t.leaves = make([]*Node, len(leaves))
	for i, leaf := range leaves {
		if len(leaf) == 0 {
			return errors.New("leaf cannot be empty")
		}

		t.leaves[i] = &Node{
			hash: t.hashFunc(leaf),
		}
	}

	// leaves are sortable
	sort.Slice(t.leaves, func(i, j int) bool {
		return bytes.Compare(t.leaves[i].hash, t.leaves[j].hash) == -1
	})

	// We add an extra empty node at the end, in case the number of leaves is odd.
	if len(t.leaves)%2 == 1 {
		t.leaves = append(t.leaves, &Node{
			hash: t.leaves[len(t.leaves)-1].hash,
		})
	}

	t.buildInternalNodes(t.leaves)

	return nil
}

func (t *MerkleTree) buildInternalNodes(nodes []*Node) {
	// we are at the root
	if len(nodes) == 1 {
		t.root = nodes[0]
		return
	}

	// the number of parents is half of the number of children
	parentNodes := make([]*Node, (len(nodes)+1)/2)
	for i := 0; i < len(nodes); i += 2 {
		// we loop in pairs, if the length of nodes is odd, left and right points to the same node
		left, right := i, i+1
		if i+1 == len(nodes) {
			right = i
		}

		// hash pair needs to be sorted
		l, r := sortPair(nodes[left].hash, nodes[right].hash)

		parent := &Node{
			hash:  t.hashFunc(l, r),
			left:  nodes[left],
			right: nodes[right],
		}
		nodes[left].parent, nodes[right].parent = parent, parent
		parentNodes[i/2] = parent
	}

	t.buildInternalNodes(parentNodes)
}

// verifyTree calculates the merkle root again by traversing the tree and verify if it's the same it holds.
func (t *MerkleTree) verifyTree() bool {
	merkleRoot := t.verify(t.root)
	return bytes.Equal(t.root.hash, merkleRoot)
}

func (t *MerkleTree) verify(node *Node) []byte {
	if node.isLeaf() {
		return node.hash
	}

	if bytes.Compare(node.left.hash, node.right.hash) > 0 {
		return t.hashFunc(t.verify(node.right), t.verify(node.left))
	}

	return t.hashFunc(t.verify(node.left), t.verify(node.right))
}

// GetProof gets the proof for a particular content.
func (t *MerkleTree) GetProof(leaf []byte) (bool, Proof) {
	index, found := sort.Find(len(t.leaves), func(i int) int {
		return bytes.Compare(leaf, t.leaves[i].hash)
	})
	if !found {
		return false, nil
	}

	l := t.leaves[index]
	var proof [][]byte
	parent := l.parent
	for parent != nil {
		if bytes.Equal(parent.left.hash, l.hash) {
			proof = append(proof, parent.right.hash)
		} else {
			proof = append(proof, parent.left.hash)
		}
		l, parent = parent, parent.parent
	}
	return true, proof
}

// MerkleRoot returns the merkle root of the tree.
func (t *MerkleTree) MerkleRoot() []byte {
	if t.root == nil {
		return nil
	}
	return t.root.hash
}

// VerifyProof verifies a given proof for a leaf.
func VerifyProof(proof Proof, root []byte, leaf []byte, hasher func() hash.Hash) bool {
	if hasher == nil {
		hasher = DefaultHasher
	}
	h := hasher()

	computedHash := leaf
	for i := 0; i < len(proof); i++ {
		left, right := sortPair(computedHash, proof[i])
		_, _ = h.Write(left)
		_, _ = h.Write(right)
		computedHash = h.Sum(nil)
		h.Reset()
	}
	return bytes.Equal(root, computedHash)
}

// Marshal serializes the tree.
//
// The encoding schema is such that the first 4 bytes we store the number of leaves (NL),
// and then the nodes' hashes are put one next to the order starting from the root
// on a level order traversal.
//
// 0       4 <----------------------- HashSize ------------------------->|
// |-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
// |   NL  |                            root                             |  ...
// |-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|-|.
func (t *MerkleTree) Marshal() []byte {
	// we start by filling the first 4 bytes with the leaves length
	data := make([]byte, EncodingSchemaNLBytes)
	binary.LittleEndian.PutUint32(data, uint32(len(t.leaves)))

	var node *Node
	queue := []*Node{t.root}
	for len(queue) > 0 {
		node, queue = queue[0], queue[1:]
		data = append(data, node.hash...)

		if node.left != nil {
			queue = append(queue, node.left)
		}

		// the second condition is for the case the node's children point to the same node
		// we don't want to duplicate the node
		if node.right != nil && node.right != node.left {
			queue = append(queue, node.right)
		}
	}

	return data
}

// Unmarshal deserializes the tree.
func Unmarshal(data []byte, hasher func() hash.Hash) (*MerkleTree, error) {
	if hasher == nil {
		hasher = DefaultHasher
	}
	h := hasher()
	hSize := h.Size()

	numberOfLeavesInBytes, data := data[0:EncodingSchemaNLBytes], data[EncodingSchemaNLBytes:]
	numberOfLeaves := int(binary.LittleEndian.Uint32(numberOfLeavesInBytes))

	if len(data)%hSize != 0 {
		return nil, errors.New("leaves data is not multiple of hash size")
	}

	// we start with an pointer p pointing to the end
	p := len(data)

	// build the leaves
	leaves := make([]*Node, numberOfLeaves)
	for i := 0; i < numberOfLeaves; i++ {
		leaves[numberOfLeaves-i-1] = &Node{
			hash: data[p-hSize*(1+i) : p-hSize*i],
		}
	}

	// adjust pointer position
	p = p - hSize*len(leaves)

	// build next levels
	previousLevelNodes := leaves
	var root *Node
	for {
		currentLevelNodes := make([]*Node, len(previousLevelNodes)/2)
		l, r := len(previousLevelNodes)-2, len(previousLevelNodes)-1

		// adjust according to length of previous level
		if len(previousLevelNodes)%2 != 0 {
			currentLevelNodes = append(currentLevelNodes, nil)
			l = r
		}

		for i := 0; i < len(currentLevelNodes); i++ {
			n := &Node{
				left:  previousLevelNodes[l],
				right: previousLevelNodes[r],
				hash:  data[p-hSize*(1+i) : p-hSize*i],
			}
			currentLevelNodes[len(currentLevelNodes)-i-1] = n
			previousLevelNodes[l].parent, previousLevelNodes[r].parent = n, n

			// adjust according to the length of previous level only on first iteration
			if i == 0 && len(previousLevelNodes)%2 != 0 {
				l, r = l-2, r-1
			} else {
				l, r = l-2, r-2
			}
		}

		// we are at the root
		if len(currentLevelNodes) == 1 {
			root = currentLevelNodes[0]
			break
		}

		p = p - hSize*len(currentLevelNodes)
		previousLevelNodes = currentLevelNodes
	}

	return &MerkleTree{
		root:   root,
		leaves: leaves,
		h:      h,
	}, nil
}

func (t *MerkleTree) hashFunc(data ...[]byte) []byte {
	t.h.Reset()
	for _, part := range data {
		_, _ = t.h.Write(part)
	}
	return t.h.Sum(nil)
}

func sortPair(a []byte, b []byte) ([]byte, []byte) {
	if bytes.Compare(a, b) > 0 {
		return b, a
	}

	return a, b
}
