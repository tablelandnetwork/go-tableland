package merkletree

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"
)

func TestNewTree(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		leaves     [][]byte
		merkleRoot []byte
	}{
		{
			"one node",
			[][]byte{[]byte("001")},
			[]byte("002"),
		},
		{
			"two nodes",
			[][]byte{[]byte("001"), []byte("002")},
			[]byte("003"),
		},
		{
			"three nodes",
			// 003 is duplicated at the end
			[][]byte{[]byte("001"), []byte("002"), []byte("003")},
			[]byte("009"),
		},
		{
			"four nodes",
			[][]byte{[]byte("001"), []byte("002"), []byte("003"), []byte("004")},
			[]byte("010"),
		},
		{
			"five nodes",
			// 005 is duplicated but does not have a power of 2 number of leaves
			[][]byte{[]byte("001"), []byte("002"), []byte("003"), []byte("004"), []byte("005")},
			[]byte("030"),
		},
		{
			"eight nodes",
			[][]byte{
				[]byte("001"), []byte("002"), []byte("003"), []byte("004"),
				[]byte("005"), []byte("006"), []byte("007"), []byte("008"),
			},
			[]byte("036"),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// shuffle leaves just to make sure the order of leaves does not affect the merkle root value
			rand.Shuffle(len(test.leaves), func(i, j int) {
				test.leaves[i], test.leaves[j] = test.leaves[j], test.leaves[i]
			})

			tree, err := NewTree(test.leaves, mockHashFunc)
			require.NoError(t, err)
			require.Equal(t, test.merkleRoot, tree.MerkleRoot())

			require.True(t, tree.verifyTree())
		})
	}

	t.Run("no leaves", func(t *testing.T) {
		t.Parallel()
		var err error

		_, err = NewTree([][]byte{}, nil)
		require.Error(t, err)

		_, err = NewTree(nil, nil)
		require.Error(t, err)
	})
}

func TestGetProof(t *testing.T) {
	t.Parallel()
	t.Run("five nodes", func(t *testing.T) {
		t.Parallel()
		leaves := [][]byte{
			[]byte("001"),
			[]byte("002"),
			[]byte("003"),
			[]byte("004"),
			[]byte("005"),
		}

		tree, err := NewTree(leaves, mockHashFunc)
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("005"), []byte("010"), []byte("010")}, tree.GetProof([]byte("005")))
	})

	t.Run("eight nodes", func(t *testing.T) {
		t.Parallel()
		leaves := [][]byte{
			[]byte("001"),
			[]byte("002"),
			[]byte("003"),
			[]byte("004"),
			[]byte("005"),
			[]byte("006"),
			[]byte("007"),
			[]byte("008"),
		}

		tree, err := NewTree(leaves, mockHashFunc)
		require.NoError(t, err)
		require.Equal(t, [][]byte{[]byte("002"), []byte("007"), []byte("026")}, tree.GetProof([]byte("001")))
		require.Equal(t, [][]byte{[]byte("001"), []byte("007"), []byte("026")}, tree.GetProof([]byte("002")))
		require.Equal(t, [][]byte{[]byte("004"), []byte("003"), []byte("026")}, tree.GetProof([]byte("003")))
		require.Equal(t, [][]byte{[]byte("003"), []byte("003"), []byte("026")}, tree.GetProof([]byte("004")))

		require.Equal(t, [][]byte{[]byte("006"), []byte("015"), []byte("010")}, tree.GetProof([]byte("005")))
		require.Equal(t, [][]byte{[]byte("005"), []byte("015"), []byte("010")}, tree.GetProof([]byte("006")))
		require.Equal(t, [][]byte{[]byte("008"), []byte("011"), []byte("010")}, tree.GetProof([]byte("007")))
		require.Equal(t, [][]byte{[]byte("007"), []byte("011"), []byte("010")}, tree.GetProof([]byte("008")))
	})

	t.Run("not found", func(t *testing.T) {
		t.Parallel()
		leaves := [][]byte{
			[]byte("001"),
			[]byte("002"),
			[]byte("003"),
			[]byte("004"),
			[]byte("005"),
		}

		tree, err := NewTree(leaves, mockHashFunc)
		require.NoError(t, err)

		require.Nil(t, tree.GetProof([]byte("006")))
		require.Len(t, tree.GetProof([]byte("006")), 0)
	})
}

func TestVerifyProof(t *testing.T) {
	t.Parallel()
	t.Run("correct proof", func(t *testing.T) {
		t.Parallel()
		root := []byte("036")
		proof := [][]byte{[]byte("002"), []byte("007"), []byte("026")}
		leaf := []byte("001")
		require.True(t, VerifyProof(root, proof, leaf, mockHashFunc))
	})

	t.Run("wrong root", func(t *testing.T) {
		t.Parallel()
		root := []byte("035")
		proof := [][]byte{[]byte("002"), []byte("007"), []byte("026")}
		leaf := []byte("001")
		require.False(t, VerifyProof(root, proof, leaf, mockHashFunc))
	})

	t.Run("wrong proof", func(t *testing.T) {
		t.Parallel()
		root := []byte("036")
		proof := [][]byte{[]byte("001"), []byte("007"), []byte("026")}
		leaf := []byte("001")
		require.False(t, VerifyProof(root, proof, leaf, mockHashFunc))
	})
}

func TestProperties(t *testing.T) {
	t.Parallel()

	// We test the properties in a bunch of hash functions to make sure the
	// kind of hash function has no influence on properties.
	hashFuncs := []func(...[]byte) []byte{
		nil,
		mockHashFunc,
		crypto.Keccak256,
		crypto.Keccak512,
		func(b ...[]byte) []byte {
			h := sha3.Sum224(b[0])
			return h[:]
		},
		func(b ...[]byte) []byte {
			h := sha3.Sum256(b[0])
			return h[:]
		},
		func(b ...[]byte) []byte {
			h := sha3.Sum384(b[0])
			return h[:]
		},
		func(b ...[]byte) []byte {
			h := sha3.Sum512(b[0])
			return h[:]
		},
		func(b ...[]byte) []byte {
			h := sha1.Sum(b[0])
			return h[:]
		},
	}

	t.Run("tree holds merkle tree property", func(t *testing.T) {
		t.Parallel()

		for _, hashFunc := range hashFuncs {
			property := func(leaves [][]byte) bool {
				if len(leaves) == 0 {
					return true
				}

				tree, err := NewTree(leaves, hashFunc)
				if err != nil {
					// ignore check when leaf is empty
					return strings.Contains(err.Error(), "leaf cannot be empty")
				}

				return tree.verifyTree()
			}
			require.NoError(t, quick.Check(property, nil))
		}
	})

	t.Run("leaves are always sorted", func(t *testing.T) {
		t.Parallel()

		for _, hashFunc := range hashFuncs {
			property := func(leaves [][]byte) bool {
				if len(leaves) == 0 {
					return true
				}

				tree, err := NewTree(leaves, hashFunc)
				if err != nil {
					// ignore check when leaf is empty
					return strings.Contains(err.Error(), "leaf cannot be empty")
				}

				return sort.SliceIsSorted(tree.leaves, func(i, j int) bool {
					return bytes.Compare(tree.leaves[i].hash, tree.leaves[j].hash) == -1
				})
			}
			require.NoError(t, quick.Check(property, nil))
		}
	})

	t.Run("height of the tree is correct", func(t *testing.T) {
		t.Parallel()
		for _, hashFunc := range hashFuncs {
			property := func(leaves [][]byte) bool {
				if len(leaves) == 0 {
					return true
				}

				tree, err := NewTree(leaves, hashFunc)
				if err != nil {
					// ignore check when leaf is empty
					return strings.Contains(err.Error(), "leaf cannot be empty")
				}

				return heightOfTree(tree.root) == expectedHeight(len(leaves))
			}
			require.NoError(t, quick.Check(property, nil))
		}
	})

	t.Run("if number of leaves is odd, then the last leaf is duplicated", func(t *testing.T) {
		t.Parallel()
		for _, hashFunc := range hashFuncs {
			property := func(leaves [][]byte) bool {
				if len(leaves)%2 == 0 {
					return true
				}

				tree, err := NewTree(leaves, hashFunc)
				if err != nil {
					// ignore check when leaf is empty
					return strings.Contains(err.Error(), "leaf cannot be empty")
				}

				return len(tree.leaves) == len(leaves)+1 &&
					bytes.Equal(tree.leaves[len(tree.leaves)-1].hash, tree.leaves[len(tree.leaves)-2].hash)
			}
			require.NoError(t, quick.Check(property, nil))
		}
	})

	t.Run("every leaf proof is correctly verifiable", func(t *testing.T) {
		t.Parallel()
		for _, hashFunc := range hashFuncs {
			property := func(leaves [][]byte) bool {
				if len(leaves) == 0 {
					return true
				}

				tree, err := NewTree(leaves, hashFunc)
				if err != nil {
					// ignore check when leaf is empty
					return strings.Contains(err.Error(), "leaf cannot be empty")
				}

				for _, leaf := range tree.leaves {
					proof := tree.GetProof(leaf.hash)
					root := tree.MerkleRoot()
					if !VerifyProof(root, proof, leaf.hash, hashFunc) {
						return false
					}
				}

				return true
			}
			require.NoError(t, quick.Check(property, nil))
		}
	})
}

// mockHashFunc is a hash function of size 3 that parses the data input as integer and sums them.
func mockHashFunc(data ...[]byte) []byte {
	var sum int64
	for _, part := range data {
		number, _ := strconv.ParseInt(string(part), 10, 0)
		sum += number
	}

	hash := fmt.Sprintf("%03d", sum)
	return []byte(hash)
}

// calculates the height of the tree.
func heightOfTree(node *Node) int {
	height := 1
	for !node.isLeaf() {
		node = node.left
		height++
	}

	return height
}

// calculates the expected height of a tree with n leaves.
func expectedHeight(n int) int {
	if n%2 == 1 {
		n++
	}

	h := 1
	for n > 1 {
		if n%2 == 1 {
			n++
		}
		h++
		n = n / 2
	}

	return h
}
