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
)

// TablelandControllerLibraryPolicy is an auto generated low-level Go binding around an user-defined struct.
type TablelandControllerLibraryPolicy struct {
	AllowInsert   bool
	AllowUpdate   bool
	AllowDelete   bool
	WhereClause   string
	UpdateColumns []string
}

// ContractMetaData contains all meta data concerning the Contract contract.
var ContractMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"caller\",\"type\":\"address\"}],\"name\":\"getPolicy\",\"outputs\":[{\"components\":[{\"internalType\":\"bool\",\"name\":\"allowInsert\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"allowUpdate\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"allowDelete\",\"type\":\"bool\"},{\"internalType\":\"string\",\"name\":\"whereClause\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"updateColumns\",\"type\":\"string[]\"}],\"internalType\":\"structTablelandControllerLibrary.Policy\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"badges\",\"type\":\"address\"}],\"name\":\"setBadges\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"rigs\",\"type\":\"address\"}],\"name\":\"setRigs\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b5061096e806100206000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c80633532ba29146100465780633791dc6a146100785780638eb4d135146100a1575b600080fd5b610076610054366004610669565b600080546001600160a01b0319166001600160a01b0392909216919091179055565b005b61008b610086366004610669565b6100d1565b6040516100989190610797565b60405180910390f35b6100766100af366004610669565b600180546001600160a01b0319166001600160a01b0392909216919091179055565b6040805160a0810182526000808252602082018190528183018190526060808301819052608083018190528351600280825291810190945291929091816020015b60608152602001906001900390816101125750506000546040805180820190915260068152651c9a59d7da5960d21b602082015291925061015e9185916001600160a01b031690610280565b816000815181106101715761017161090c565b60200260200101819052506101b683600160009054906101000a90046001600160a01b0316604051806040016040528060028152602001611a5960f21b815250610280565b816001815181106101c9576101c961090c565b6020908102919091010152604080516001808252818301909252600091816020015b60608152602001906001900390816101eb579050509050604051806040016040528060088152602001673837b9b4ba34b7b760c11b815250816000815181106102365761023661090c565b60200260200101819052506040518060a00160405280600015158152602001600115158152602001600015158152602001610270846104a8565b8152602001919091529392505050565b6040516370a0823160e01b81526001600160a01b0384811660048301526060918491600091908316906370a082319060240160206040518083038186803b1580156102ca57600080fd5b505afa1580156102de573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906103029190610699565b9050600081116103585760405162461bcd60e51b815260206004820181905260248201527f726571756972654f6e654f664552433732313a20756e617574686f72697a6564604482015260640160405180910390fd5b60008460405160200161036b919061070d565b604051602081830303815290604052905060005b8281101561047b57604051632f745c5960e01b81526001600160a01b0389811660048301526024820183905260009161041391871690632f745c599060440160206040518083038186803b1580156103d657600080fd5b505afa1580156103ea573d6000803e3d6000fd5b505050506040513d601f19601f8201168201806040525081019061040e9190610699565b610563565b90508161044357828160405160200161042d9291906106de565b6040516020818303038152906040529250610468565b8281604051602001610456929190610736565b60405160208183030381529060405292505b5080610473816108b1565b91505061037f565b508060405160200161048d9190610772565b60408051808303601f19018152919052979650505050505050565b60608060005b835181101561055c57818482815181106104ca576104ca61090c565b60200260200101516040516020016104e39291906106de565b604051602081830303815290604052915060018451610502919061086a565b811461054a57816040518060400160405280600581526020016401030b732160dd1b8152506040516020016105389291906106de565b60405160208183030381529060405291505b80610554816108b1565b9150506104ae565b5092915050565b6060816105875750506040805180820190915260018152600360fc1b602082015290565b8160005b81156105b1578061059b816108b1565b91506105aa9050600a83610856565b915061058b565b60008167ffffffffffffffff8111156105cc576105cc610922565b6040519080825280601f01601f1916602001820160405280156105f6576020820181803683370190505b5090505b84156106615761060b60018361086a565b9150610618600a866108cc565b61062390603061083e565b60f81b8183815181106106385761063861090c565b60200101906001600160f81b031916908160001a90535061065a600a86610856565b94506105fa565b949350505050565b60006020828403121561067b57600080fd5b81356001600160a01b038116811461069257600080fd5b9392505050565b6000602082840312156106ab57600080fd5b5051919050565b600081518084526106ca816020860160208601610881565b601f01601f19169290920160200192915050565b600083516106f0818460208801610881565b835190830190610704818360208801610881565b01949350505050565b6000825161071f818460208701610881565b64040d2dc40560db1b920191825250600501919050565b60008351610748818460208801610881565b600b60fa1b9083019081528351610766816001840160208801610881565b01600101949350505050565b60008251610784818460208701610881565b602960f81b920191825250600101919050565b6000602080835283511515818401528084015115156040840152604084015115156060840152606084015160a060808501526107d660c08501826106b2565b6080860151601f19868303810160a088015281518084529293509084019184840190600581901b8501860160005b82811015610830578487830301845261081e8287516106b2565b95880195938801939150600101610804565b509998505050505050505050565b60008219821115610851576108516108e0565b500190565b600082610865576108656108f6565b500490565b60008282101561087c5761087c6108e0565b500390565b60005b8381101561089c578181015183820152602001610884565b838111156108ab576000848401525b50505050565b60006000198214156108c5576108c56108e0565b5060010190565b6000826108db576108db6108f6565b500690565b634e487b7160e01b600052601160045260246000fd5b634e487b7160e01b600052601260045260246000fd5b634e487b7160e01b600052603260045260246000fd5b634e487b7160e01b600052604160045260246000fdfea26469706673582212209596580bca0e9ccf9600b711bedfb458e02abd4362cff5e2455fb68b56c0806d64736f6c63430008070033",
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

// GetPolicy is a free data retrieval call binding the contract method 0x3791dc6a.
//
// Solidity: function getPolicy(address caller) view returns((bool,bool,bool,string,string[]))
func (_Contract *ContractCaller) GetPolicy(opts *bind.CallOpts, caller common.Address) (TablelandControllerLibraryPolicy, error) {
	var out []interface{}
	err := _Contract.contract.Call(opts, &out, "getPolicy", caller)

	if err != nil {
		return *new(TablelandControllerLibraryPolicy), err
	}

	out0 := *abi.ConvertType(out[0], new(TablelandControllerLibraryPolicy)).(*TablelandControllerLibraryPolicy)

	return out0, err

}

// GetPolicy is a free data retrieval call binding the contract method 0x3791dc6a.
//
// Solidity: function getPolicy(address caller) view returns((bool,bool,bool,string,string[]))
func (_Contract *ContractSession) GetPolicy(caller common.Address) (TablelandControllerLibraryPolicy, error) {
	return _Contract.Contract.GetPolicy(&_Contract.CallOpts, caller)
}

// GetPolicy is a free data retrieval call binding the contract method 0x3791dc6a.
//
// Solidity: function getPolicy(address caller) view returns((bool,bool,bool,string,string[]))
func (_Contract *ContractCallerSession) GetPolicy(caller common.Address) (TablelandControllerLibraryPolicy, error) {
	return _Contract.Contract.GetPolicy(&_Contract.CallOpts, caller)
}

// SetBadges is a paid mutator transaction binding the contract method 0x8eb4d135.
//
// Solidity: function setBadges(address badges) returns()
func (_Contract *ContractTransactor) SetBadges(opts *bind.TransactOpts, badges common.Address) (*types.Transaction, error) {
	return _Contract.contract.Transact(opts, "setBadges", badges)
}

// SetBadges is a paid mutator transaction binding the contract method 0x8eb4d135.
//
// Solidity: function setBadges(address badges) returns()
func (_Contract *ContractSession) SetBadges(badges common.Address) (*types.Transaction, error) {
	return _Contract.Contract.SetBadges(&_Contract.TransactOpts, badges)
}

// SetBadges is a paid mutator transaction binding the contract method 0x8eb4d135.
//
// Solidity: function setBadges(address badges) returns()
func (_Contract *ContractTransactorSession) SetBadges(badges common.Address) (*types.Transaction, error) {
	return _Contract.Contract.SetBadges(&_Contract.TransactOpts, badges)
}

// SetRigs is a paid mutator transaction binding the contract method 0x3532ba29.
//
// Solidity: function setRigs(address rigs) returns()
func (_Contract *ContractTransactor) SetRigs(opts *bind.TransactOpts, rigs common.Address) (*types.Transaction, error) {
	return _Contract.contract.Transact(opts, "setRigs", rigs)
}

// SetRigs is a paid mutator transaction binding the contract method 0x3532ba29.
//
// Solidity: function setRigs(address rigs) returns()
func (_Contract *ContractSession) SetRigs(rigs common.Address) (*types.Transaction, error) {
	return _Contract.Contract.SetRigs(&_Contract.TransactOpts, rigs)
}

// SetRigs is a paid mutator transaction binding the contract method 0x3532ba29.
//
// Solidity: function setRigs(address rigs) returns()
func (_Contract *ContractTransactorSession) SetRigs(rigs common.Address) (*types.Transaction, error) {
	return _Contract.Contract.SetRigs(&_Contract.TransactOpts, rigs)
}
