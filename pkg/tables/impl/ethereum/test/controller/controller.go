// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package controller

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
	_ = abi.ConvertType
)

// ITablelandControllerPolicy is an auto generated low-level Go binding around an user-defined struct.
type ITablelandControllerPolicy struct {
	AllowInsert      bool
	AllowUpdate      bool
	AllowDelete      bool
	WhereClause      string
	WithCheck        string
	UpdatableColumns []string
}

// ContractMetaData contains all meta data concerning the Contract contract.
var ContractMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[],\"name\":\"ERC721AQueryablePoliciesUnauthorized\",\"type\":\"error\"},{\"inputs\":[],\"name\":\"ERC721EnumerablePoliciesUnauthorized\",\"type\":\"error\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"caller\",\"type\":\"address\"}],\"name\":\"getPolicy\",\"outputs\":[{\"components\":[{\"internalType\":\"bool\",\"name\":\"allowInsert\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"allowUpdate\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"allowDelete\",\"type\":\"bool\"},{\"internalType\":\"string\",\"name\":\"whereClause\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"withCheck\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"updatableColumns\",\"type\":\"string[]\"}],\"internalType\":\"structITablelandController.Policy\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"bars\",\"type\":\"address\"}],\"name\":\"setBars\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"foos\",\"type\":\"address\"}],\"name\":\"setFoos\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b5061001a3361001f565b61006f565b600080546001600160a01b038381166001600160a01b0319831681178455604051919092169283917f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e09190a35050565b610fb18061007e6000396000f3fe608060405234801561001057600080fd5b50600436106100625760003560e01c80633791dc6a14610067578063715018a61461009057806383f9a5dc1461009a5780638da5cb5b146100ad578063cefd9818146100c8578063f2fde38b146100db575b600080fd5b61007a610075366004610bb6565b6100ee565b6040516100879190610dd6565b60405180910390f35b6100986103d0565b005b6100986100a8366004610bb6565b61040f565b6000546040516001600160a01b039091168152602001610087565b6100986100d6366004610bb6565b61045b565b6100986100e9366004610bb6565b6104a7565b6040805160c08101825260008082526020820181905281830181905260608083018190526080830181905260a083018190528351600280825291810190945291929091816020015b60608152602001906001900390816101365750506040805160038082526080820190925291925060009190602082015b6060815260200190600190039081610166575050600154604080518082019091526006815265199bdbd7da5960d21b60208201529192506101b29186916001600160a01b031690610542565b826000815181106101d357634e487b7160e01b600052603260045260246000fd5b602002602001018190525061021c84600260009054906101000a90046001600160a01b03166040518060400160405280600681526020016518985c97da5960d21b815250610734565b8260018151811061023d57634e487b7160e01b600052603260045260246000fd5b6020908102919091010152604080516001808252818301909252600091816020015b606081526020019060019003908161025f579050509050604051806040016040528060038152602001623130bd60e91b815250816000815181106102b357634e487b7160e01b600052603260045260246000fd5b602002602001018190525060405180602001604052806000815250826000815181106102ef57634e487b7160e01b600052603260045260246000fd5b602002602001018190525060405180604001604052806007815260200166062617a203e20360cc1b8152508260018151811061033b57634e487b7160e01b600052603260045260246000fd5b6020026020010181905250604051806020016040528060008152508260028151811061037757634e487b7160e01b600052603260045260246000fd5b60200260200101819052506040518060c001604052806000151581526020016001151581526020016000151581526020016103b185610951565b81526020016103bf84610951565b815260200191909152949350505050565b6000546001600160a01b031633146104035760405162461bcd60e51b81526004016103fa90610da1565b60405180910390fd5b61040d6000610a44565b565b6000546001600160a01b031633146104395760405162461bcd60e51b81526004016103fa90610da1565b600280546001600160a01b0319166001600160a01b0392909216919091179055565b6000546001600160a01b031633146104855760405162461bcd60e51b81526004016103fa90610da1565b600180546001600160a01b0319166001600160a01b0392909216919091179055565b6000546001600160a01b031633146104d15760405162461bcd60e51b81526004016103fa90610da1565b6001600160a01b0381166105365760405162461bcd60e51b815260206004820152602660248201527f4f776e61626c653a206e6577206f776e657220697320746865207a65726f206160448201526564647265737360d01b60648201526084016103fa565b61053f81610a44565b50565b6040516370a0823160e01b81526001600160a01b0384811660048301526060918491600091908316906370a082319060240160206040518083038186803b15801561058c57600080fd5b505afa1580156105a0573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906105c49190610ca4565b9050806105e457604051630b61338f60e11b815260040160405180910390fd5b6000846040516020016105f79190610d17565b604051602081830303815290604052905060005b8281101561070757604051632f745c5960e01b81526001600160a01b0389811660048301526024820183905260009161069f91871690632f745c599060440160206040518083038186803b15801561066257600080fd5b505afa158015610676573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061069a9190610ca4565b610a94565b9050816106cf5782816040516020016106b9929190610ce8565b60405160208183030381529060405292506106f4565b82816040516020016106e2929190610d40565b60405160208183030381529060405292505b50806106ff81610f0a565b91505061060b565b50806040516020016107199190610d7c565b60408051808303601f19018152919052979650505050505050565b6040516370a0823160e01b81526001600160a01b0384811660048301526060918491600091908316906370a082319060240160206040518083038186803b15801561077e57600080fd5b505afa158015610792573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906107b69190610ca4565b9050806107d657604051633e72a67f60e01b815260040160405180910390fd5b604051632118854760e21b81526001600160a01b03878116600483015260009190841690638462151c9060240160006040518083038186803b15801561081b57600080fd5b505afa15801561082f573d6000803e3d6000fd5b505050506040513d6000823e601f3d908101601f191682016040526108579190810190610be4565b905060008560405160200161086c9190610d17565b604051602081830303815290604052905060005b82518110156109235760006108bb8483815181106108ae57634e487b7160e01b600052603260045260246000fd5b6020026020010151610a94565b9050816108eb5782816040516020016108d5929190610ce8565b6040516020818303038152906040529250610910565b82816040516020016108fe929190610d40565b60405160208183030381529060405292505b508061091b81610f0a565b915050610880565b50806040516020016109359190610d7c565b60408051808303601f1901815291905298975050505050505050565b60608060005b8351811015610a3d5783818151811061098057634e487b7160e01b600052603260045260246000fd5b6020026020010151516000141561099657610a2b565b8151156109df57816040518060400160405280600581526020016401030b732160dd1b8152506040516020016109cd929190610ce8565b60405160208183030381529060405291505b81848281518110610a0057634e487b7160e01b600052603260045260246000fd5b6020026020010151604051602001610a19929190610ce8565b60405160208183030381529060405291505b80610a3581610f0a565b915050610957565b5092915050565b600080546001600160a01b038381166001600160a01b0319831681178455604051919092169283917f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e09190a35050565b606081610ab85750506040805180820190915260018152600360fc1b602082015290565b8160005b8115610ae25780610acc81610f0a565b9150610adb9050600a83610eaf565b9150610abc565b60008167ffffffffffffffff811115610b0b57634e487b7160e01b600052604160045260246000fd5b6040519080825280601f01601f191660200182016040528015610b35576020820181803683370190505b5090505b8415610bae57610b4a600183610ec3565b9150610b57600a86610f25565b610b62906030610e97565b60f81b818381518110610b8557634e487b7160e01b600052603260045260246000fd5b60200101906001600160f81b031916908160001a905350610ba7600a86610eaf565b9450610b39565b949350505050565b600060208284031215610bc7578081fd5b81356001600160a01b0381168114610bdd578182fd5b9392505050565b60006020808385031215610bf6578182fd5b825167ffffffffffffffff80821115610c0d578384fd5b818501915085601f830112610c20578384fd5b815181811115610c3257610c32610f65565b8060051b604051601f19603f83011681018181108582111715610c5757610c57610f65565b604052828152858101935084860182860187018a1015610c75578788fd5b8795505b83861015610c97578051855260019590950194938601938601610c79565b5098975050505050505050565b600060208284031215610cb5578081fd5b5051919050565b60008151808452610cd4816020860160208601610eda565b601f01601f19169290920160200192915050565b60008351610cfa818460208801610eda565b835190830190610d0e818360208801610eda565b01949350505050565b60008251610d29818460208701610eda565b64040d2dc40560db1b920191825250600501919050565b60008351610d52818460208801610eda565b600b60fa1b9083019081528351610d70816001840160208801610eda565b01600101949350505050565b60008251610d8e818460208701610eda565b602960f81b920191825250600101919050565b6020808252818101527f4f776e61626c653a2063616c6c6572206973206e6f7420746865206f776e6572604082015260600190565b6000602080835283511515818401528084015115156040840152604084015115156060840152606084015160c06080850152610e1560e0850182610cbc565b90506080850151601f19808684030160a0870152610e338383610cbc565b60a0880151878203830160c089015280518083529194508501925084840190600581901b85018601875b82811015610e895784878303018452610e77828751610cbc565b95880195938801939150600101610e5d565b509998505050505050505050565b60008219821115610eaa57610eaa610f39565b500190565b600082610ebe57610ebe610f4f565b500490565b600082821015610ed557610ed5610f39565b500390565b60005b83811015610ef5578181015183820152602001610edd565b83811115610f04576000848401525b50505050565b6000600019821415610f1e57610f1e610f39565b5060010190565b600082610f3457610f34610f4f565b500690565b634e487b7160e01b600052601160045260246000fd5b634e487b7160e01b600052601260045260246000fd5b634e487b7160e01b600052604160045260246000fdfea26469706673582212200576687ef10c0d26f1bdc6a74b6b11b7b9b29032fe81ac311496de902f2690a964736f6c63430008040033",
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
	parsed, err := ContractMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
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

// GetPolicy is a free data retrieval call binding the contract method 0x3791dc6a.
//
// Solidity: function getPolicy(address caller) view returns((bool,bool,bool,string,string,string[]))
func (_Contract *ContractCaller) GetPolicy(opts *bind.CallOpts, caller common.Address) (ITablelandControllerPolicy, error) {
	var out []interface{}
	err := _Contract.contract.Call(opts, &out, "getPolicy", caller)

	if err != nil {
		return *new(ITablelandControllerPolicy), err
	}

	out0 := *abi.ConvertType(out[0], new(ITablelandControllerPolicy)).(*ITablelandControllerPolicy)

	return out0, err

}

// GetPolicy is a free data retrieval call binding the contract method 0x3791dc6a.
//
// Solidity: function getPolicy(address caller) view returns((bool,bool,bool,string,string,string[]))
func (_Contract *ContractSession) GetPolicy(caller common.Address) (ITablelandControllerPolicy, error) {
	return _Contract.Contract.GetPolicy(&_Contract.CallOpts, caller)
}

// GetPolicy is a free data retrieval call binding the contract method 0x3791dc6a.
//
// Solidity: function getPolicy(address caller) view returns((bool,bool,bool,string,string,string[]))
func (_Contract *ContractCallerSession) GetPolicy(caller common.Address) (ITablelandControllerPolicy, error) {
	return _Contract.Contract.GetPolicy(&_Contract.CallOpts, caller)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Contract *ContractCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _Contract.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Contract *ContractSession) Owner() (common.Address, error) {
	return _Contract.Contract.Owner(&_Contract.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_Contract *ContractCallerSession) Owner() (common.Address, error) {
	return _Contract.Contract.Owner(&_Contract.CallOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Contract *ContractTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Contract.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Contract *ContractSession) RenounceOwnership() (*types.Transaction, error) {
	return _Contract.Contract.RenounceOwnership(&_Contract.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_Contract *ContractTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _Contract.Contract.RenounceOwnership(&_Contract.TransactOpts)
}

// SetBars is a paid mutator transaction binding the contract method 0x83f9a5dc.
//
// Solidity: function setBars(address bars) returns()
func (_Contract *ContractTransactor) SetBars(opts *bind.TransactOpts, bars common.Address) (*types.Transaction, error) {
	return _Contract.contract.Transact(opts, "setBars", bars)
}

// SetBars is a paid mutator transaction binding the contract method 0x83f9a5dc.
//
// Solidity: function setBars(address bars) returns()
func (_Contract *ContractSession) SetBars(bars common.Address) (*types.Transaction, error) {
	return _Contract.Contract.SetBars(&_Contract.TransactOpts, bars)
}

// SetBars is a paid mutator transaction binding the contract method 0x83f9a5dc.
//
// Solidity: function setBars(address bars) returns()
func (_Contract *ContractTransactorSession) SetBars(bars common.Address) (*types.Transaction, error) {
	return _Contract.Contract.SetBars(&_Contract.TransactOpts, bars)
}

// SetFoos is a paid mutator transaction binding the contract method 0xcefd9818.
//
// Solidity: function setFoos(address foos) returns()
func (_Contract *ContractTransactor) SetFoos(opts *bind.TransactOpts, foos common.Address) (*types.Transaction, error) {
	return _Contract.contract.Transact(opts, "setFoos", foos)
}

// SetFoos is a paid mutator transaction binding the contract method 0xcefd9818.
//
// Solidity: function setFoos(address foos) returns()
func (_Contract *ContractSession) SetFoos(foos common.Address) (*types.Transaction, error) {
	return _Contract.Contract.SetFoos(&_Contract.TransactOpts, foos)
}

// SetFoos is a paid mutator transaction binding the contract method 0xcefd9818.
//
// Solidity: function setFoos(address foos) returns()
func (_Contract *ContractTransactorSession) SetFoos(foos common.Address) (*types.Transaction, error) {
	return _Contract.Contract.SetFoos(&_Contract.TransactOpts, foos)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Contract *ContractTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _Contract.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Contract *ContractSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Contract.Contract.TransferOwnership(&_Contract.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_Contract *ContractTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _Contract.Contract.TransferOwnership(&_Contract.TransactOpts, newOwner)
}

// ContractOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the Contract contract.
type ContractOwnershipTransferredIterator struct {
	Event *ContractOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *ContractOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(ContractOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(ContractOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *ContractOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *ContractOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// ContractOwnershipTransferred represents a OwnershipTransferred event raised by the Contract contract.
type ContractOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Contract *ContractFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*ContractOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Contract.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &ContractOwnershipTransferredIterator{contract: _Contract.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Contract *ContractFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *ContractOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _Contract.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(ContractOwnershipTransferred)
				if err := _Contract.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_Contract *ContractFilterer) ParseOwnershipTransferred(log types.Log) (*ContractOwnershipTransferred, error) {
	event := new(ContractOwnershipTransferred)
	if err := _Contract.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
