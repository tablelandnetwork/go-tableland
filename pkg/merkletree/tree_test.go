package merkletree

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"hash"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/sha3"
)

func TestNewTree(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name       string
		leaves     [][]byte
		merkleRoot []byte
		serialized []byte
	}{
		{
			"one node",
			[][]byte{[]byte("001")},
			[]byte("002"),
			[]byte("002001001"),
		},
		{
			"two nodes",
			[][]byte{[]byte("001"), []byte("002")},
			[]byte("003"),
			[]byte("003001002"),
		},
		{
			"three nodes",
			// 003 is duplicated at the end
			[][]byte{[]byte("001"), []byte("002"), []byte("003")},
			[]byte("009"),
			[]byte("009003006001002003003"),
		},
		{
			"four nodes",
			[][]byte{[]byte("001"), []byte("002"), []byte("003"), []byte("004")},
			[]byte("010"),
			[]byte("010003007001002003004"),
		},
		{
			"five nodes",
			// 005 is duplicated but does not have a power of 2 number of leaves
			[][]byte{[]byte("001"), []byte("002"), []byte("003"), []byte("004"), []byte("005")},
			[]byte("030"),
			[]byte("030010020003007010001002003004005005"),
		},
		{
			"eight nodes",
			[][]byte{
				[]byte("001"), []byte("002"), []byte("003"), []byte("004"),
				[]byte("005"), []byte("006"), []byte("007"), []byte("008"),
			},
			[]byte("036"),
			[]byte("036010026003007011015001002003004005006007008"),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// shuffle leaves just to make sure the order of leaves does not affect the merkle root value
			rand.Shuffle(len(test.leaves), func(i, j int) {
				test.leaves[i], test.leaves[j] = test.leaves[j], test.leaves[i]
			})

			tree, err := NewTree(test.leaves, mockHash)
			require.NoError(t, err)
			require.Equal(t, test.merkleRoot, tree.MerkleRoot())

			s := tree.Marshal()
			require.Equal(t, test.serialized, s[EncodingSchemaNLBytes:])
			require.Equal(t, len(tree.leaves), int(binary.LittleEndian.Uint32(s[:EncodingSchemaNLBytes])))
			require.Len(t, s, EncodingSchemaNLBytes+expectedNumberOfNodes(len(tree.leaves))*mockHash().Size())
			require.True(t, tree.verifyTree())

			// check that we get a tree equal to the original
			tree2, err := Unmarshal(s, mockHash)
			require.NoError(t, err)
			require.True(t, tree2.verifyTree())
			require.Equal(t, s, tree2.Marshal())
			require.Equal(t, tree, tree2)
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

		tree, err := NewTree(leaves, mockHash)
		require.NoError(t, err)

		found, proof := tree.GetProof([]byte("005"))
		require.True(t, found)
		require.Equal(t, Proof([][]byte{[]byte("005"), []byte("010"), []byte("010")}), proof)
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

		tree, err := NewTree(leaves, mockHash)
		require.NoError(t, err)

		found, proof := tree.GetProof([]byte("001"))
		require.True(t, found)
		require.Equal(t, Proof([][]byte{[]byte("002"), []byte("007"), []byte("026")}), proof)

		found, proof = tree.GetProof([]byte("002"))
		require.True(t, found)
		require.Equal(t, Proof([][]byte{[]byte("001"), []byte("007"), []byte("026")}), proof)

		found, proof = tree.GetProof([]byte("003"))
		require.True(t, found)
		require.Equal(t, Proof([][]byte{[]byte("004"), []byte("003"), []byte("026")}), proof)

		found, proof = tree.GetProof([]byte("004"))
		require.True(t, found)
		require.Equal(t, Proof([][]byte{[]byte("003"), []byte("003"), []byte("026")}), proof)

		found, proof = tree.GetProof([]byte("005"))
		require.True(t, found)
		require.Equal(t, Proof([][]byte{[]byte("006"), []byte("015"), []byte("010")}), proof)

		found, proof = tree.GetProof([]byte("006"))
		require.True(t, found)
		require.Equal(t, Proof([][]byte{[]byte("005"), []byte("015"), []byte("010")}), proof)

		found, proof = tree.GetProof([]byte("007"))
		require.True(t, found)
		require.Equal(t, Proof([][]byte{[]byte("008"), []byte("011"), []byte("010")}), proof)

		found, proof = tree.GetProof([]byte("008"))
		require.True(t, found)
		require.Equal(t, Proof([][]byte{[]byte("007"), []byte("011"), []byte("010")}), proof)
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

		tree, err := NewTree(leaves, mockHash)
		require.NoError(t, err)

		found, proof := tree.GetProof([]byte("006"))
		require.False(t, found)
		require.Nil(t, proof)
		require.Len(t, proof, 0)
	})
}

func TestVerifyProof(t *testing.T) {
	t.Parallel()
	t.Run("correct proof", func(t *testing.T) {
		t.Parallel()
		root := []byte("036")
		proof := Proof([][]byte{[]byte("002"), []byte("007"), []byte("026")})
		leaf := []byte("001")
		require.True(t, VerifyProof(proof, root, leaf, mockHash))
	})

	t.Run("wrong root", func(t *testing.T) {
		t.Parallel()
		root := []byte("035")
		proof := Proof([][]byte{[]byte("002"), []byte("007"), []byte("026")})
		leaf := []byte("001")
		require.False(t, VerifyProof(proof, root, leaf, mockHash))
	})

	t.Run("wrong proof", func(t *testing.T) {
		t.Parallel()
		root := []byte("036")
		proof := Proof([][]byte{[]byte("001"), []byte("007"), []byte("026")})
		leaf := []byte("001")
		require.False(t, VerifyProof(proof, root, leaf, mockHash))
	})
}

func TestProperties(t *testing.T) {
	t.Parallel()

	// We test the properties in a bunch of hash functions to make sure the
	// kind of hash function has no influence on properties.
	hashers := []func() hash.Hash{
		nil,
		func() hash.Hash { return &mockHasher{} },
		sha3.NewLegacyKeccak256,
		sha3.NewLegacyKeccak512,
		sha3.New224,
		sha3.New256,
		sha3.New384,
		sha3.New512,
		sha1.New,
	}

	t.Run("tree holds merkle tree property", func(t *testing.T) {
		t.Parallel()

		for _, hasher := range hashers {
			property := func(leaves [][]byte) bool {
				if len(leaves) == 0 {
					return true
				}

				tree, err := NewTree(leaves, hasher)
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

		for _, hasher := range hashers {
			property := func(leaves [][]byte) bool {
				if len(leaves) == 0 {
					return true
				}

				tree, err := NewTree(leaves, hasher)
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
		for _, hasher := range hashers {
			property := func(leaves [][]byte) bool {
				if len(leaves) == 0 {
					return true
				}

				tree, err := NewTree(leaves, hasher)
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
		for _, hasher := range hashers {
			property := func(leaves [][]byte) bool {
				if len(leaves)%2 == 0 {
					return true
				}

				tree, err := NewTree(leaves, hasher)
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
		for _, hasher := range hashers {
			property := func(leaves [][]byte) bool {
				if len(leaves) == 0 {
					return true
				}

				tree, err := NewTree(leaves, hasher)
				if err != nil {
					// ignore check when leaf is empty
					return strings.Contains(err.Error(), "leaf cannot be empty")
				}

				for _, leaf := range tree.leaves {
					_, proof := tree.GetProof(leaf.hash)
					root := tree.MerkleRoot()
					if !VerifyProof(proof, root, leaf.hash, hasher) {
						return false
					}
				}

				return true
			}
			require.NoError(t, quick.Check(property, nil))
		}
	})

	t.Run("serializing then deserializing does not change the tree", func(t *testing.T) {
		t.Parallel()
		for _, hasher := range hashers {
			property := func(leaves [][]byte) bool {
				if len(leaves) == 0 {
					return true
				}

				tree, err := NewTree(leaves, hasher)
				if err != nil {
					// ignore check when leaf is empty
					return strings.Contains(err.Error(), "leaf cannot be empty")
				}
				require.True(t, tree.verifyTree())

				s := tree.Marshal()
				tree2, err := Unmarshal(s, hasher)
				require.NoError(t, err)

				require.True(t, tree2.verifyTree())
				require.Equal(t, s, tree2.Marshal())
				require.Equal(t, tree.root, tree2.root)

				return true
			}
			require.NoError(t, quick.Check(property, nil))
		}
	})
}

func mockHash() hash.Hash {
	return &mockHasher{}
}

type mockHasher struct {
	sum int64
}

func (h *mockHasher) Write(p []byte) (n int, err error) {
	number, _ := strconv.ParseInt(string(p), 10, 0)
	h.sum += number
	return len(p), nil
}

func (h *mockHasher) Sum(_ []byte) []byte {
	hash := fmt.Sprintf("%03d", h.sum)
	return []byte(hash)
}

func (h *mockHasher) Reset() {
	h.sum = 0
}

func (h *mockHasher) Size() int {
	return 3
}

func (h *mockHasher) BlockSize() int {
	return 0
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

// calculates the expected number of the nodes of a tree with n leaves.
func expectedNumberOfNodes(n int) int {
	nodes := n
	for n > 1 {
		if n%2 == 1 {
			n++
		}
		n = n / 2
		nodes = nodes + n
	}

	return nodes
}
