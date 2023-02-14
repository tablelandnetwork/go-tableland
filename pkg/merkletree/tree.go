package merkletree

import (
	"bytes"
	"errors"
	"fmt"
	"sort"

	"github.com/ethereum/go-ethereum/crypto"
)

// DefaultHashFunc is the default hash function in case none is passed.
var DefaultHashFunc = crypto.Keccak256

// MerkleTree is a binary Merkle Tree implemenation.
type MerkleTree struct {
	root   *Node
	leaves []*Node

	hashFunc func(...[]byte) []byte
}

// Node represents a Node of MerkleTree.
type Node struct {
	parent, left, right *Node
	hash                []byte
}

func (n *Node) isLeaf() bool {
	return n.left == nil && n.right == nil
}

// NewTree builds a new Merkle Tree.
func NewTree(leaves [][]byte, hashFunc func(...[]byte) []byte) (*MerkleTree, error) {
	if hashFunc == nil {
		hashFunc = DefaultHashFunc
	}

	tree := &MerkleTree{
		hashFunc: hashFunc,
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
// It returns `nil` if the leaf is not present in the tree.
func (t *MerkleTree) GetProof(leaf []byte) [][]byte {
	index, found := sort.Find(len(t.leaves), func(i int) int {
		return bytes.Compare(leaf, t.leaves[i].hash)
	})
	if !found {
		return nil
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
	return proof
}

// VerifyProof verifies a given proof for a leaf.
func VerifyProof(root []byte, proof [][]byte, leaf []byte, hashFunc func(data ...[]byte) []byte) bool {
	if hashFunc == nil {
		hashFunc = DefaultHashFunc
	}

	computedHash := leaf
	for i := 0; i < len(proof); i++ {
		left, right := sortPair(computedHash, proof[i])
		computedHash = hashFunc(left, right)
	}
	return bytes.Equal(root, computedHash)
}

// MerkleRoot returns the merkle root of the tree.
func (t *MerkleTree) MerkleRoot() []byte {
	if t.root == nil {
		return nil
	}
	return t.root.hash
}

func sortPair(a []byte, b []byte) ([]byte, []byte) {
	if bytes.Compare(a, b) > 0 {
		return b, a
	}

	return a, b
}
