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
	AllowInsert      bool
	AllowUpdate      bool
	AllowDelete      bool
	WhereClause      string
	WhereCheck       string
	UpdatableColumns []string
}

// ContractMetaData contains all meta data concerning the Contract contract.
var ContractMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"caller\",\"type\":\"address\"}],\"name\":\"getPolicy\",\"outputs\":[{\"components\":[{\"internalType\":\"bool\",\"name\":\"allowInsert\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"allowUpdate\",\"type\":\"bool\"},{\"internalType\":\"bool\",\"name\":\"allowDelete\",\"type\":\"bool\"},{\"internalType\":\"string\",\"name\":\"whereClause\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"whereCheck\",\"type\":\"string\"},{\"internalType\":\"string[]\",\"name\":\"updatableColumns\",\"type\":\"string[]\"}],\"internalType\":\"structTablelandControllerLibrary.Policy\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"badges\",\"type\":\"address\"}],\"name\":\"setBadges\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"rigs\",\"type\":\"address\"}],\"name\":\"setRigs\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
	Bin: "0x608060405234801561001057600080fd5b50610fbf806100206000396000f3fe608060405234801561001057600080fd5b50600436106100415760003560e01c80633532ba29146100465780633791dc6a146100625780638eb4d13514610092575b600080fd5b610060600480360381019061005b9190610865565b6100ae565b005b61007c60048036038101906100779190610865565b6100f1565b6040516100899190610c12565b60405180910390f35b6100ac60048036038101906100a79190610865565b61034a565b005b806000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b6100f96107ff565b6000600267ffffffffffffffff81111561011657610115610eed565b5b60405190808252806020026020018201604052801561014957816020015b60608152602001906001900390816101345790505b5090506101ac8360008054906101000a900473ffffffffffffffffffffffffffffffffffffffff166040518060400160405280600681526020017f7269675f6964000000000000000000000000000000000000000000000000000081525061038e565b816000815181106101c0576101bf610ebe565b5b602002602001018190525061022d83600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff166040518060400160405280600281526020017f696400000000000000000000000000000000000000000000000000000000000081525061038e565b8160018151811061024157610240610ebe565b5b60200260200101819052506000600167ffffffffffffffff81111561026957610268610eed565b5b60405190808252806020026020018201604052801561029c57816020015b60608152602001906001900390816102875790505b5090506040518060400160405280600881526020017f706f736974696f6e000000000000000000000000000000000000000000000000815250816000815181106102e9576102e8610ebe565b5b60200260200101819052506040518060c00160405280600015158152602001600115158152602001600015158152602001610323846105c6565b81526020016040518060200160405280600081525081526020018281525092505050919050565b80600160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b6060600083905060008173ffffffffffffffffffffffffffffffffffffffff166370a08231876040518263ffffffff1660e01b81526004016103d09190610bae565b60206040518083038186803b1580156103e857600080fd5b505afa1580156103fc573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906104209190610892565b905060008111610465576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161045c90610bf2565b60405180910390fd5b6000846040516020016104789190610b2f565b604051602081830303815290604052905060005b828110156105965760006105298573ffffffffffffffffffffffffffffffffffffffff16632f745c598b856040518363ffffffff1660e01b81526004016104d4929190610bc9565b60206040518083038186803b1580156104ec57600080fd5b505afa158015610500573d6000803e3d6000fd5b505050506040513d601f19601f820116820180604052508101906105249190610892565b61069e565b9050600082141561055d578281604051602001610547929190610b0b565b6040516020818303038152906040529250610582565b8281604051602001610570929190610b55565b60405160208183030381529060405292505b50808061058e90610de6565b91505061048c565b50806040516020016105a89190610b88565b60405160208183030381529060405290508093505050509392505050565b60608060005b835181101561069457818482815181106105e9576105e8610ebe565b5b6020026020010151604051602001610602929190610b0b565b6040516020818303038152906040529150600184516106219190610d37565b811461068157816040518060400160405280600581526020017f20616e642000000000000000000000000000000000000000000000000000000081525060405160200161066f929190610b0b565b60405160208183030381529060405291505b808061068c90610de6565b9150506105cc565b5080915050919050565b606060008214156106e6576040518060400160405280600181526020017f300000000000000000000000000000000000000000000000000000000000000081525090506107fa565b600082905060005b6000821461071857808061070190610de6565b915050600a826107119190610d06565b91506106ee565b60008167ffffffffffffffff81111561073457610733610eed565b5b6040519080825280601f01601f1916602001820160405280156107665781602001600182028036833780820191505090505b5090505b600085146107f35760018261077f9190610d37565b9150600a8561078e9190610e2f565b603061079a9190610cb0565b60f81b8183815181106107b0576107af610ebe565b5b60200101907effffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff1916908160001a905350600a856107ec9190610d06565b945061076a565b8093505050505b919050565b6040518060c001604052806000151581526020016000151581526020016000151581526020016060815260200160608152602001606081525090565b60008135905061084a81610f5b565b92915050565b60008151905061085f81610f72565b92915050565b60006020828403121561087b5761087a610f1c565b5b60006108898482850161083b565b91505092915050565b6000602082840312156108a8576108a7610f1c565b5b60006108b684828501610850565b91505092915050565b60006108cb8383610997565b905092915050565b6108dc81610d6b565b82525050565b60006108ed82610c44565b6108f78185610c72565b93508360208202850161090985610c34565b8060005b85811015610945578484038952815161092685826108bf565b945061093183610c65565b925060208a0199505060018101905061090d565b50829750879550505050505092915050565b61096081610d7d565b82525050565b600061097182610c4f565b61097b8185610c83565b935061098b818560208601610db3565b80840191505092915050565b60006109a282610c5a565b6109ac8185610c8e565b93506109bc818560208601610db3565b6109c581610f21565b840191505092915050565b7f20696e2028000000000000000000000000000000000000000000000000000000815250565b7f2c00000000000000000000000000000000000000000000000000000000000000815250565b7f2900000000000000000000000000000000000000000000000000000000000000815250565b6000610a4f602083610c9f565b9150610a5a82610f32565b602082019050919050565b600060c083016000830151610a7d6000860182610957565b506020830151610a906020860182610957565b506040830151610aa36040860182610957565b5060608301518482036060860152610abb8282610997565b91505060808301518482036080860152610ad58282610997565b91505060a083015184820360a0860152610aef82826108e2565b9150508091505092915050565b610b0581610da9565b82525050565b6000610b178285610966565b9150610b238284610966565b91508190509392505050565b6000610b3b8284610966565b9150610b46826109d0565b60058201915081905092915050565b6000610b618285610966565b9150610b6c826109f6565b600182019150610b7c8284610966565b91508190509392505050565b6000610b948284610966565b9150610b9f82610a1c565b60018201915081905092915050565b6000602082019050610bc360008301846108d3565b92915050565b6000604082019050610bde60008301856108d3565b610beb6020830184610afc565b9392505050565b60006020820190508181036000830152610c0b81610a42565b9050919050565b60006020820190508181036000830152610c2c8184610a65565b905092915050565b6000819050602082019050919050565b600081519050919050565b600081519050919050565b600081519050919050565b6000602082019050919050565b600082825260208201905092915050565b600081905092915050565b600082825260208201905092915050565b600082825260208201905092915050565b6000610cbb82610da9565b9150610cc683610da9565b9250827fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff03821115610cfb57610cfa610e60565b5b828201905092915050565b6000610d1182610da9565b9150610d1c83610da9565b925082610d2c57610d2b610e8f565b5b828204905092915050565b6000610d4282610da9565b9150610d4d83610da9565b925082821015610d6057610d5f610e60565b5b828203905092915050565b6000610d7682610d89565b9050919050565b60008115159050919050565b600073ffffffffffffffffffffffffffffffffffffffff82169050919050565b6000819050919050565b60005b83811015610dd1578082015181840152602081019050610db6565b83811115610de0576000848401525b50505050565b6000610df182610da9565b91507fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff821415610e2457610e23610e60565b5b600182019050919050565b6000610e3a82610da9565b9150610e4583610da9565b925082610e5557610e54610e8f565b5b828206905092915050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b600080fd5b6000601f19601f8301169050919050565b7f726571756972654f6e654f664552433732313a20756e617574686f72697a6564600082015250565b610f6481610d6b565b8114610f6f57600080fd5b50565b610f7b81610da9565b8114610f8657600080fd5b5056fea2646970667358221220dd6730251a75d18ffb5dd13ce7ba5694c72e328bde80b0331fed63dd41c4d73264736f6c63430008070033",
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
// Solidity: function getPolicy(address caller) view returns((bool,bool,bool,string,string,string[]))
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
// Solidity: function getPolicy(address caller) view returns((bool,bool,bool,string,string,string[]))
func (_Contract *ContractSession) GetPolicy(caller common.Address) (TablelandControllerLibraryPolicy, error) {
	return _Contract.Contract.GetPolicy(&_Contract.CallOpts, caller)
}

// GetPolicy is a free data retrieval call binding the contract method 0x3791dc6a.
//
// Solidity: function getPolicy(address caller) view returns((bool,bool,bool,string,string,string[]))
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
