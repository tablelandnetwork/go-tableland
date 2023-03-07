// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package impl

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
)

// ContractMetaData contains all meta data concerning the Contract contract.
var ContractMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"uint256[]\",\"name\":\"tableIds\",\"type\":\"uint256[]\"},{\"internalType\":\"bytes32[]\",\"name\":\"roots\",\"type\":\"bytes32[]\"}],\"name\":\"setRoots\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"tableId\",\"type\":\"uint256\"},{\"internalType\":\"bytes32\",\"name\":\"row\",\"type\":\"bytes32\"},{\"internalType\":\"bytes32[]\",\"name\":\"proof\",\"type\":\"bytes32[]\"}],\"name\":\"verifyRowInclusion\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b50610613806100206000396000f3fe608060405234801561001057600080fd5b50600436106100365760003560e01c8063719f98111461003b578063d88fd1f41461006b575b600080fd5b61005560048036038101906100509190610339565b610087565b60405161006291906103c8565b60405180910390f35b61008560048036038101906100809190610439565b6100f6565b005b6000806000808781526020019081526020016000205490506100eb848480806020026020016040519081016040528093929190818152602001838360200280828437600081840152601f19601f8201169050808301925050505050505082876101af565b915050949350505050565b81819050848490501461013e576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161013590610517565b60405180910390fd5b60005b848490508110156101a85782828281811061015f5761015e610537565b5b9050602002013560008087878581811061017c5761017b610537565b5b9050602002013581526020019081526020016000208190555080806101a090610595565b915050610141565b5050505050565b6000826101bc85846101c6565b1490509392505050565b60008082905060005b8451811015610211576101fc828683815181106101ef576101ee610537565b5b602002602001015161021c565b9150808061020990610595565b9150506101cf565b508091505092915050565b60008183106102345761022f8284610247565b61023f565b61023e8383610247565b5b905092915050565b600082600052816020526040600020905092915050565b600080fd5b600080fd5b6000819050919050565b61027b81610268565b811461028657600080fd5b50565b60008135905061029881610272565b92915050565b6000819050919050565b6102b18161029e565b81146102bc57600080fd5b50565b6000813590506102ce816102a8565b92915050565b600080fd5b600080fd5b600080fd5b60008083601f8401126102f9576102f86102d4565b5b8235905067ffffffffffffffff811115610316576103156102d9565b5b602083019150836020820283011115610332576103316102de565b5b9250929050565b600080600080606085870312156103535761035261025e565b5b600061036187828801610289565b9450506020610372878288016102bf565b935050604085013567ffffffffffffffff81111561039357610392610263565b5b61039f878288016102e3565b925092505092959194509250565b60008115159050919050565b6103c2816103ad565b82525050565b60006020820190506103dd60008301846103b9565b92915050565b60008083601f8401126103f9576103f86102d4565b5b8235905067ffffffffffffffff811115610416576104156102d9565b5b602083019150836020820283011115610432576104316102de565b5b9250929050565b600080600080604085870312156104535761045261025e565b5b600085013567ffffffffffffffff81111561047157610470610263565b5b61047d878288016103e3565b9450945050602085013567ffffffffffffffff8111156104a05761049f610263565b5b6104ac878288016102e3565b925092505092959194509250565b600082825260208201905092915050565b7f6c656e6774687320646f6e2774206d6174636800000000000000000000000000600082015250565b60006105016013836104ba565b915061050c826104cb565b602082019050919050565b60006020820190508181036000830152610530816104f4565b9050919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b60006105a082610268565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff82036105d2576105d1610566565b5b60018201905091905056fea2646970667358221220db32a5f9744ccb3526456fbb36b8152b9bed84903b501d1c160ed16fbb41600964736f6c63430008120033",
}

// ContractABI is the input ABI used to generate the binding from.
// Deprecated: Use ContractMetaData.ABI instead.
var ContractABI = ContractMetaData.ABI

// ContractBin is the compiled bytecode used for deploying new contracts.
// Deprecated: Use ContractMetaData.Bin instead.
var ContractBin = ContractMetaData.Bin

// DeployContract deploys a new Ethereum contract, binding an instance of Contract to it.
func DeployContract(auth *bind.TransactOpts, backend bind.ContractBackend) (common.Address, *types.Transaction, *Contract, error) {
	parsed, err := ContractMetaData.GetAbi()
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	if parsed == nil {
		return common.Address{}, nil, nil, errors.New("GetABI returned nil")
	}

	address, tx, contract, err := bind.DeployContract(auth, *parsed, common.FromHex(ContractBin), backend)
	if err != nil {
		return common.Address{}, nil, nil, err
	}
	return address, tx, &Contract{ContractCaller: ContractCaller{contract: contract}, ContractTransactor: ContractTransactor{contract: contract}, ContractFilterer: ContractFilterer{contract: contract}}, nil
}

// Contract is an auto generated Go binding around an Ethereum contract.
type Contract struct {
	ContractCaller     // Read-only binding to the contract
	ContractTransactor // Write-only binding to the contract
	ContractFilterer   // Log filterer for contract events
}

// ContractCaller is an auto generated read-only Go binding around an Ethereum contract.
type ContractCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ContractTransactor is an auto generated write-only Go binding around an Ethereum contract.
type ContractTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ContractFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type ContractFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// ContractSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type ContractSession struct {
	Contract     *Contract         // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// ContractCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type ContractCallerSession struct {
	Contract *ContractCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts   // Call options to use throughout this session
}

// ContractTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type ContractTransactorSession struct {
	Contract     *ContractTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts   // Transaction auth options to use throughout this session
}

// ContractRaw is an auto generated low-level Go binding around an Ethereum contract.
type ContractRaw struct {
	Contract *Contract // Generic contract binding to access the raw methods on
}

// ContractCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type ContractCallerRaw struct {
	Contract *ContractCaller // Generic read-only contract binding to access the raw methods on
}

// ContractTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type ContractTransactorRaw struct {
	Contract *ContractTransactor // Generic write-only contract binding to access the raw methods on
}

// NewContract creates a new instance of Contract, bound to a specific deployed contract.
func NewContract(address common.Address, backend bind.ContractBackend) (*Contract, error) {
	contract, err := bindContract(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Contract{ContractCaller: ContractCaller{contract: contract}, ContractTransactor: ContractTransactor{contract: contract}, ContractFilterer: ContractFilterer{contract: contract}}, nil
}

// NewContractCaller creates a new read-only instance of Contract, bound to a specific deployed contract.
func NewContractCaller(address common.Address, caller bind.ContractCaller) (*ContractCaller, error) {
	contract, err := bindContract(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &ContractCaller{contract: contract}, nil
}

// NewContractTransactor creates a new write-only instance of Contract, bound to a specific deployed contract.
func NewContractTransactor(address common.Address, transactor bind.ContractTransactor) (*ContractTransactor, error) {
	contract, err := bindContract(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &ContractTransactor{contract: contract}, nil
}

// NewContractFilterer creates a new log filterer instance of Contract, bound to a specific deployed contract.
func NewContractFilterer(address common.Address, filterer bind.ContractFilterer) (*ContractFilterer, error) {
	contract, err := bindContract(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &ContractFilterer{contract: contract}, nil
}

// bindContract binds a generic wrapper to an already deployed contract.
func bindContract(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(ContractABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Contract *ContractRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Contract.Contract.ContractCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Contract *ContractRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Contract.Contract.ContractTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Contract *ContractRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Contract.Contract.ContractTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Contract *ContractCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Contract.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Contract *ContractTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Contract.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Contract *ContractTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Contract.Contract.contract.Transact(opts, method, params...)
}

// VerifyRowInclusion is a free data retrieval call binding the contract method 0x719f9811.
//
// Solidity: function verifyRowInclusion(uint256 tableId, bytes32 row, bytes32[] proof) view returns(bool)
func (_Contract *ContractCaller) VerifyRowInclusion(opts *bind.CallOpts, tableId *big.Int, row [32]byte, proof [][32]byte) (bool, error) {
	var out []interface{}
	err := _Contract.contract.Call(opts, &out, "verifyRowInclusion", tableId, row, proof)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// VerifyRowInclusion is a free data retrieval call binding the contract method 0x719f9811.
//
// Solidity: function verifyRowInclusion(uint256 tableId, bytes32 row, bytes32[] proof) view returns(bool)
func (_Contract *ContractSession) VerifyRowInclusion(tableId *big.Int, row [32]byte, proof [][32]byte) (bool, error) {
	return _Contract.Contract.VerifyRowInclusion(&_Contract.CallOpts, tableId, row, proof)
}

// VerifyRowInclusion is a free data retrieval call binding the contract method 0x719f9811.
//
// Solidity: function verifyRowInclusion(uint256 tableId, bytes32 row, bytes32[] proof) view returns(bool)
func (_Contract *ContractCallerSession) VerifyRowInclusion(tableId *big.Int, row [32]byte, proof [][32]byte) (bool, error) {
	return _Contract.Contract.VerifyRowInclusion(&_Contract.CallOpts, tableId, row, proof)
}

// SetRoots is a paid mutator transaction binding the contract method 0xd88fd1f4.
//
// Solidity: function setRoots(uint256[] tableIds, bytes32[] roots) returns()
func (_Contract *ContractTransactor) SetRoots(opts *bind.TransactOpts, tableIds []*big.Int, roots [][32]byte) (*types.Transaction, error) {
	return _Contract.contract.Transact(opts, "setRoots", tableIds, roots)
}

// SetRoots is a paid mutator transaction binding the contract method 0xd88fd1f4.
//
// Solidity: function setRoots(uint256[] tableIds, bytes32[] roots) returns()
func (_Contract *ContractSession) SetRoots(tableIds []*big.Int, roots [][32]byte) (*types.Transaction, error) {
	return _Contract.Contract.SetRoots(&_Contract.TransactOpts, tableIds, roots)
}

// SetRoots is a paid mutator transaction binding the contract method 0xd88fd1f4.
//
// Solidity: function setRoots(uint256[] tableIds, bytes32[] roots) returns()
func (_Contract *ContractTransactorSession) SetRoots(tableIds []*big.Int, roots [][32]byte) (*types.Transaction, error) {
	return _Contract.Contract.SetRoots(&_Contract.TransactOpts, tableIds, roots)
}
