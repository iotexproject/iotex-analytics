// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package contract

import (
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// DelegateProfileABI is the input ABI used to generate the binding from.
const DelegateProfileABI = "[{\"constant\":false,\"inputs\":[{\"name\":\"_delegate\",\"type\":\"address\"},{\"name\":\"_name\",\"type\":\"string\"},{\"name\":\"_value\",\"type\":\"bytes\"}],\"name\":\"updateProfileForDelegate\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"register\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegate\",\"type\":\"address\"}],\"name\":\"getEncodedProfile\",\"outputs\":[{\"name\":\"code_\",\"type\":\"bytes\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_address\",\"type\":\"address\"}],\"name\":\"isOwner\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_name\",\"type\":\"string\"}],\"name\":\"getFieldByName\",\"outputs\":[{\"name\":\"verifier_\",\"type\":\"address\"},{\"name\":\"deprecated_\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_byteCode\",\"type\":\"bytes\"}],\"name\":\"updateProfileWithByteCode\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"withdraw\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"unpause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"paused\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_name\",\"type\":\"string\"},{\"name\":\"_verifierAddr\",\"type\":\"address\"}],\"name\":\"newField\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_name\",\"type\":\"string\"},{\"name\":\"_value\",\"type\":\"bytes\"}],\"name\":\"updateProfile\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[],\"name\":\"pause\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_idx\",\"type\":\"uint256\"}],\"name\":\"getFieldByIndex\",\"outputs\":[{\"name\":\"name_\",\"type\":\"string\"},{\"name\":\"verifier_\",\"type\":\"address\"},{\"name\":\"deprecated_\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"name\":\"\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_delegate\",\"type\":\"address\"},{\"name\":\"_byteCode\",\"type\":\"bytes\"}],\"name\":\"updateProfileWithByteCodeForDelegate\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"name\":\"fieldNames\",\"outputs\":[{\"name\":\"\",\"type\":\"string\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_addr\",\"type\":\"address\"}],\"name\":\"registered\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[{\"name\":\"_delegate\",\"type\":\"address\"},{\"name\":\"_field\",\"type\":\"string\"}],\"name\":\"getProfileByField\",\"outputs\":[{\"name\":\"\",\"type\":\"bytes\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_name\",\"type\":\"string\"}],\"name\":\"deprecateField\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"constant\":true,\"inputs\":[],\"name\":\"numOfFields\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"},{\"constant\":false,\"inputs\":[{\"name\":\"_newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"name\":\"registerAddr\",\"type\":\"address\"}],\"payable\":false,\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"fee\",\"type\":\"uint256\"}],\"name\":\"FeeUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"delegate\",\"type\":\"address\"},{\"indexed\":false,\"name\":\"name\",\"type\":\"string\"},{\"indexed\":false,\"name\":\"value\",\"type\":\"bytes\"}],\"name\":\"ProfileUpdated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"name\",\"type\":\"string\"}],\"name\":\"FieldDeprecated\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"name\":\"name\",\"type\":\"string\"}],\"name\":\"NewField\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Pause\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[],\"name\":\"Unpause\",\"type\":\"event\"}]"

// DelegateProfile is an auto generated Go binding around an Ethereum contract.
type DelegateProfile struct {
	DelegateProfileCaller     // Read-only binding to the contract
	DelegateProfileTransactor // Write-only binding to the contract
	DelegateProfileFilterer   // Log filterer for contract events
}

// DelegateProfileCaller is an auto generated read-only Go binding around an Ethereum contract.
type DelegateProfileCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DelegateProfileTransactor is an auto generated write-only Go binding around an Ethereum contract.
type DelegateProfileTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DelegateProfileFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type DelegateProfileFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// DelegateProfileSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type DelegateProfileSession struct {
	Contract     *DelegateProfile  // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// DelegateProfileCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type DelegateProfileCallerSession struct {
	Contract *DelegateProfileCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts          // Call options to use throughout this session
}

// DelegateProfileTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type DelegateProfileTransactorSession struct {
	Contract     *DelegateProfileTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts          // Transaction auth options to use throughout this session
}

// DelegateProfileRaw is an auto generated low-level Go binding around an Ethereum contract.
type DelegateProfileRaw struct {
	Contract *DelegateProfile // Generic contract binding to access the raw methods on
}

// DelegateProfileCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type DelegateProfileCallerRaw struct {
	Contract *DelegateProfileCaller // Generic read-only contract binding to access the raw methods on
}

// DelegateProfileTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type DelegateProfileTransactorRaw struct {
	Contract *DelegateProfileTransactor // Generic write-only contract binding to access the raw methods on
}

// NewDelegateProfile creates a new instance of DelegateProfile, bound to a specific deployed contract.
func NewDelegateProfile(address common.Address, backend bind.ContractBackend) (*DelegateProfile, error) {
	contract, err := bindDelegateProfile(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &DelegateProfile{DelegateProfileCaller: DelegateProfileCaller{contract: contract}, DelegateProfileTransactor: DelegateProfileTransactor{contract: contract}, DelegateProfileFilterer: DelegateProfileFilterer{contract: contract}}, nil
}

// NewDelegateProfileCaller creates a new read-only instance of DelegateProfile, bound to a specific deployed contract.
func NewDelegateProfileCaller(address common.Address, caller bind.ContractCaller) (*DelegateProfileCaller, error) {
	contract, err := bindDelegateProfile(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &DelegateProfileCaller{contract: contract}, nil
}

// NewDelegateProfileTransactor creates a new write-only instance of DelegateProfile, bound to a specific deployed contract.
func NewDelegateProfileTransactor(address common.Address, transactor bind.ContractTransactor) (*DelegateProfileTransactor, error) {
	contract, err := bindDelegateProfile(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &DelegateProfileTransactor{contract: contract}, nil
}

// NewDelegateProfileFilterer creates a new log filterer instance of DelegateProfile, bound to a specific deployed contract.
func NewDelegateProfileFilterer(address common.Address, filterer bind.ContractFilterer) (*DelegateProfileFilterer, error) {
	contract, err := bindDelegateProfile(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &DelegateProfileFilterer{contract: contract}, nil
}

// bindDelegateProfile binds a generic wrapper to an already deployed contract.
func bindDelegateProfile(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := abi.JSON(strings.NewReader(DelegateProfileABI))
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DelegateProfile *DelegateProfileRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _DelegateProfile.Contract.DelegateProfileCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DelegateProfile *DelegateProfileRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DelegateProfile.Contract.DelegateProfileTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DelegateProfile *DelegateProfileRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DelegateProfile.Contract.DelegateProfileTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_DelegateProfile *DelegateProfileCallerRaw) Call(opts *bind.CallOpts, result interface{}, method string, params ...interface{}) error {
	return _DelegateProfile.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_DelegateProfile *DelegateProfileTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DelegateProfile.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_DelegateProfile *DelegateProfileTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _DelegateProfile.Contract.contract.Transact(opts, method, params...)
}

// FieldNames is a free data retrieval call binding the contract method 0xb2d3dd66.
//
// Solidity: function fieldNames( uint256) constant returns(string)
func (_DelegateProfile *DelegateProfileCaller) FieldNames(opts *bind.CallOpts, arg0 *big.Int) (string, error) {
	var (
		ret0 = new(string)
	)
	out := ret0
	err := _DelegateProfile.contract.Call(opts, out, "fieldNames", arg0)
	return *ret0, err
}

// FieldNames is a free data retrieval call binding the contract method 0xb2d3dd66.
//
// Solidity: function fieldNames( uint256) constant returns(string)
func (_DelegateProfile *DelegateProfileSession) FieldNames(arg0 *big.Int) (string, error) {
	return _DelegateProfile.Contract.FieldNames(&_DelegateProfile.CallOpts, arg0)
}

// FieldNames is a free data retrieval call binding the contract method 0xb2d3dd66.
//
// Solidity: function fieldNames( uint256) constant returns(string)
func (_DelegateProfile *DelegateProfileCallerSession) FieldNames(arg0 *big.Int) (string, error) {
	return _DelegateProfile.Contract.FieldNames(&_DelegateProfile.CallOpts, arg0)
}

// GetEncodedProfile is a free data retrieval call binding the contract method 0x2652877e.
//
// Solidity: function getEncodedProfile(_delegate address) constant returns(code_ bytes)
func (_DelegateProfile *DelegateProfileCaller) GetEncodedProfile(opts *bind.CallOpts, _delegate common.Address) ([]byte, error) {
	var (
		ret0 = new([]byte)
	)
	out := ret0
	err := _DelegateProfile.contract.Call(opts, out, "getEncodedProfile", _delegate)
	return *ret0, err
}

// GetEncodedProfile is a free data retrieval call binding the contract method 0x2652877e.
//
// Solidity: function getEncodedProfile(_delegate address) constant returns(code_ bytes)
func (_DelegateProfile *DelegateProfileSession) GetEncodedProfile(_delegate common.Address) ([]byte, error) {
	return _DelegateProfile.Contract.GetEncodedProfile(&_DelegateProfile.CallOpts, _delegate)
}

// GetEncodedProfile is a free data retrieval call binding the contract method 0x2652877e.
//
// Solidity: function getEncodedProfile(_delegate address) constant returns(code_ bytes)
func (_DelegateProfile *DelegateProfileCallerSession) GetEncodedProfile(_delegate common.Address) ([]byte, error) {
	return _DelegateProfile.Contract.GetEncodedProfile(&_DelegateProfile.CallOpts, _delegate)
}

// GetFieldByIndex is a free data retrieval call binding the contract method 0x8ac834a3.
//
// Solidity: function getFieldByIndex(_idx uint256) constant returns(name_ string, verifier_ address, deprecated_ bool)
func (_DelegateProfile *DelegateProfileCaller) GetFieldByIndex(opts *bind.CallOpts, _idx *big.Int) (struct {
	Name       string
	Verifier   common.Address
	Deprecated bool
}, error) {
	ret := new(struct {
		Name       string
		Verifier   common.Address
		Deprecated bool
	})
	out := ret
	err := _DelegateProfile.contract.Call(opts, out, "getFieldByIndex", _idx)
	return *ret, err
}

// GetFieldByIndex is a free data retrieval call binding the contract method 0x8ac834a3.
//
// Solidity: function getFieldByIndex(_idx uint256) constant returns(name_ string, verifier_ address, deprecated_ bool)
func (_DelegateProfile *DelegateProfileSession) GetFieldByIndex(_idx *big.Int) (struct {
	Name       string
	Verifier   common.Address
	Deprecated bool
}, error) {
	return _DelegateProfile.Contract.GetFieldByIndex(&_DelegateProfile.CallOpts, _idx)
}

// GetFieldByIndex is a free data retrieval call binding the contract method 0x8ac834a3.
//
// Solidity: function getFieldByIndex(_idx uint256) constant returns(name_ string, verifier_ address, deprecated_ bool)
func (_DelegateProfile *DelegateProfileCallerSession) GetFieldByIndex(_idx *big.Int) (struct {
	Name       string
	Verifier   common.Address
	Deprecated bool
}, error) {
	return _DelegateProfile.Contract.GetFieldByIndex(&_DelegateProfile.CallOpts, _idx)
}

// GetFieldByName is a free data retrieval call binding the contract method 0x363d62dd.
//
// Solidity: function getFieldByName(_name string) constant returns(verifier_ address, deprecated_ bool)
func (_DelegateProfile *DelegateProfileCaller) GetFieldByName(opts *bind.CallOpts, _name string) (struct {
	Verifier   common.Address
	Deprecated bool
}, error) {
	ret := new(struct {
		Verifier   common.Address
		Deprecated bool
	})
	out := ret
	err := _DelegateProfile.contract.Call(opts, out, "getFieldByName", _name)
	return *ret, err
}

// GetFieldByName is a free data retrieval call binding the contract method 0x363d62dd.
//
// Solidity: function getFieldByName(_name string) constant returns(verifier_ address, deprecated_ bool)
func (_DelegateProfile *DelegateProfileSession) GetFieldByName(_name string) (struct {
	Verifier   common.Address
	Deprecated bool
}, error) {
	return _DelegateProfile.Contract.GetFieldByName(&_DelegateProfile.CallOpts, _name)
}

// GetFieldByName is a free data retrieval call binding the contract method 0x363d62dd.
//
// Solidity: function getFieldByName(_name string) constant returns(verifier_ address, deprecated_ bool)
func (_DelegateProfile *DelegateProfileCallerSession) GetFieldByName(_name string) (struct {
	Verifier   common.Address
	Deprecated bool
}, error) {
	return _DelegateProfile.Contract.GetFieldByName(&_DelegateProfile.CallOpts, _name)
}

// GetProfileByField is a free data retrieval call binding the contract method 0xcdcb1d52.
//
// Solidity: function getProfileByField(_delegate address, _field string) constant returns(bytes)
func (_DelegateProfile *DelegateProfileCaller) GetProfileByField(opts *bind.CallOpts, _delegate common.Address, _field string) ([]byte, error) {
	var (
		ret0 = new([]byte)
	)
	out := ret0
	err := _DelegateProfile.contract.Call(opts, out, "getProfileByField", _delegate, _field)
	return *ret0, err
}

// GetProfileByField is a free data retrieval call binding the contract method 0xcdcb1d52.
//
// Solidity: function getProfileByField(_delegate address, _field string) constant returns(bytes)
func (_DelegateProfile *DelegateProfileSession) GetProfileByField(_delegate common.Address, _field string) ([]byte, error) {
	return _DelegateProfile.Contract.GetProfileByField(&_DelegateProfile.CallOpts, _delegate, _field)
}

// GetProfileByField is a free data retrieval call binding the contract method 0xcdcb1d52.
//
// Solidity: function getProfileByField(_delegate address, _field string) constant returns(bytes)
func (_DelegateProfile *DelegateProfileCallerSession) GetProfileByField(_delegate common.Address, _field string) ([]byte, error) {
	return _DelegateProfile.Contract.GetProfileByField(&_DelegateProfile.CallOpts, _delegate, _field)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(_address address) constant returns(bool)
func (_DelegateProfile *DelegateProfileCaller) IsOwner(opts *bind.CallOpts, _address common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _DelegateProfile.contract.Call(opts, out, "isOwner", _address)
	return *ret0, err
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(_address address) constant returns(bool)
func (_DelegateProfile *DelegateProfileSession) IsOwner(_address common.Address) (bool, error) {
	return _DelegateProfile.Contract.IsOwner(&_DelegateProfile.CallOpts, _address)
}

// IsOwner is a free data retrieval call binding the contract method 0x2f54bf6e.
//
// Solidity: function isOwner(_address address) constant returns(bool)
func (_DelegateProfile *DelegateProfileCallerSession) IsOwner(_address common.Address) (bool, error) {
	return _DelegateProfile.Contract.IsOwner(&_DelegateProfile.CallOpts, _address)
}

// NumOfFields is a free data retrieval call binding the contract method 0xe6ce112f.
//
// Solidity: function numOfFields() constant returns(uint256)
func (_DelegateProfile *DelegateProfileCaller) NumOfFields(opts *bind.CallOpts) (*big.Int, error) {
	var (
		ret0 = new(*big.Int)
	)
	out := ret0
	err := _DelegateProfile.contract.Call(opts, out, "numOfFields")
	return *ret0, err
}

// NumOfFields is a free data retrieval call binding the contract method 0xe6ce112f.
//
// Solidity: function numOfFields() constant returns(uint256)
func (_DelegateProfile *DelegateProfileSession) NumOfFields() (*big.Int, error) {
	return _DelegateProfile.Contract.NumOfFields(&_DelegateProfile.CallOpts)
}

// NumOfFields is a free data retrieval call binding the contract method 0xe6ce112f.
//
// Solidity: function numOfFields() constant returns(uint256)
func (_DelegateProfile *DelegateProfileCallerSession) NumOfFields() (*big.Int, error) {
	return _DelegateProfile.Contract.NumOfFields(&_DelegateProfile.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_DelegateProfile *DelegateProfileCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _DelegateProfile.contract.Call(opts, out, "owner")
	return *ret0, err
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_DelegateProfile *DelegateProfileSession) Owner() (common.Address, error) {
	return _DelegateProfile.Contract.Owner(&_DelegateProfile.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() constant returns(address)
func (_DelegateProfile *DelegateProfileCallerSession) Owner() (common.Address, error) {
	return _DelegateProfile.Contract.Owner(&_DelegateProfile.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_DelegateProfile *DelegateProfileCaller) Paused(opts *bind.CallOpts) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _DelegateProfile.contract.Call(opts, out, "paused")
	return *ret0, err
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_DelegateProfile *DelegateProfileSession) Paused() (bool, error) {
	return _DelegateProfile.Contract.Paused(&_DelegateProfile.CallOpts)
}

// Paused is a free data retrieval call binding the contract method 0x5c975abb.
//
// Solidity: function paused() constant returns(bool)
func (_DelegateProfile *DelegateProfileCallerSession) Paused() (bool, error) {
	return _DelegateProfile.Contract.Paused(&_DelegateProfile.CallOpts)
}

// Register is a free data retrieval call binding the contract method 0x1aa3a008.
//
// Solidity: function register() constant returns(address)
func (_DelegateProfile *DelegateProfileCaller) Register(opts *bind.CallOpts) (common.Address, error) {
	var (
		ret0 = new(common.Address)
	)
	out := ret0
	err := _DelegateProfile.contract.Call(opts, out, "register")
	return *ret0, err
}

// Register is a free data retrieval call binding the contract method 0x1aa3a008.
//
// Solidity: function register() constant returns(address)
func (_DelegateProfile *DelegateProfileSession) Register() (common.Address, error) {
	return _DelegateProfile.Contract.Register(&_DelegateProfile.CallOpts)
}

// Register is a free data retrieval call binding the contract method 0x1aa3a008.
//
// Solidity: function register() constant returns(address)
func (_DelegateProfile *DelegateProfileCallerSession) Register() (common.Address, error) {
	return _DelegateProfile.Contract.Register(&_DelegateProfile.CallOpts)
}

// Registered is a free data retrieval call binding the contract method 0xb2dd5c07.
//
// Solidity: function registered(_addr address) constant returns(bool)
func (_DelegateProfile *DelegateProfileCaller) Registered(opts *bind.CallOpts, _addr common.Address) (bool, error) {
	var (
		ret0 = new(bool)
	)
	out := ret0
	err := _DelegateProfile.contract.Call(opts, out, "registered", _addr)
	return *ret0, err
}

// Registered is a free data retrieval call binding the contract method 0xb2dd5c07.
//
// Solidity: function registered(_addr address) constant returns(bool)
func (_DelegateProfile *DelegateProfileSession) Registered(_addr common.Address) (bool, error) {
	return _DelegateProfile.Contract.Registered(&_DelegateProfile.CallOpts, _addr)
}

// Registered is a free data retrieval call binding the contract method 0xb2dd5c07.
//
// Solidity: function registered(_addr address) constant returns(bool)
func (_DelegateProfile *DelegateProfileCallerSession) Registered(_addr common.Address) (bool, error) {
	return _DelegateProfile.Contract.Registered(&_DelegateProfile.CallOpts, _addr)
}

// DeprecateField is a paid mutator transaction binding the contract method 0xe0adf839.
//
// Solidity: function deprecateField(_name string) returns()
func (_DelegateProfile *DelegateProfileTransactor) DeprecateField(opts *bind.TransactOpts, _name string) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "deprecateField", _name)
}

// DeprecateField is a paid mutator transaction binding the contract method 0xe0adf839.
//
// Solidity: function deprecateField(_name string) returns()
func (_DelegateProfile *DelegateProfileSession) DeprecateField(_name string) (*types.Transaction, error) {
	return _DelegateProfile.Contract.DeprecateField(&_DelegateProfile.TransactOpts, _name)
}

// DeprecateField is a paid mutator transaction binding the contract method 0xe0adf839.
//
// Solidity: function deprecateField(_name string) returns()
func (_DelegateProfile *DelegateProfileTransactorSession) DeprecateField(_name string) (*types.Transaction, error) {
	return _DelegateProfile.Contract.DeprecateField(&_DelegateProfile.TransactOpts, _name)
}

// NewField is a paid mutator transaction binding the contract method 0x68beafc8.
//
// Solidity: function newField(_name string, _verifierAddr address) returns()
func (_DelegateProfile *DelegateProfileTransactor) NewField(opts *bind.TransactOpts, _name string, _verifierAddr common.Address) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "newField", _name, _verifierAddr)
}

// NewField is a paid mutator transaction binding the contract method 0x68beafc8.
//
// Solidity: function newField(_name string, _verifierAddr address) returns()
func (_DelegateProfile *DelegateProfileSession) NewField(_name string, _verifierAddr common.Address) (*types.Transaction, error) {
	return _DelegateProfile.Contract.NewField(&_DelegateProfile.TransactOpts, _name, _verifierAddr)
}

// NewField is a paid mutator transaction binding the contract method 0x68beafc8.
//
// Solidity: function newField(_name string, _verifierAddr address) returns()
func (_DelegateProfile *DelegateProfileTransactorSession) NewField(_name string, _verifierAddr common.Address) (*types.Transaction, error) {
	return _DelegateProfile.Contract.NewField(&_DelegateProfile.TransactOpts, _name, _verifierAddr)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_DelegateProfile *DelegateProfileTransactor) Pause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "pause")
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_DelegateProfile *DelegateProfileSession) Pause() (*types.Transaction, error) {
	return _DelegateProfile.Contract.Pause(&_DelegateProfile.TransactOpts)
}

// Pause is a paid mutator transaction binding the contract method 0x8456cb59.
//
// Solidity: function pause() returns()
func (_DelegateProfile *DelegateProfileTransactorSession) Pause() (*types.Transaction, error) {
	return _DelegateProfile.Contract.Pause(&_DelegateProfile.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(_newOwner address) returns()
func (_DelegateProfile *DelegateProfileTransactor) TransferOwnership(opts *bind.TransactOpts, _newOwner common.Address) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "transferOwnership", _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(_newOwner address) returns()
func (_DelegateProfile *DelegateProfileSession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _DelegateProfile.Contract.TransferOwnership(&_DelegateProfile.TransactOpts, _newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(_newOwner address) returns()
func (_DelegateProfile *DelegateProfileTransactorSession) TransferOwnership(_newOwner common.Address) (*types.Transaction, error) {
	return _DelegateProfile.Contract.TransferOwnership(&_DelegateProfile.TransactOpts, _newOwner)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_DelegateProfile *DelegateProfileTransactor) Unpause(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "unpause")
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_DelegateProfile *DelegateProfileSession) Unpause() (*types.Transaction, error) {
	return _DelegateProfile.Contract.Unpause(&_DelegateProfile.TransactOpts)
}

// Unpause is a paid mutator transaction binding the contract method 0x3f4ba83a.
//
// Solidity: function unpause() returns()
func (_DelegateProfile *DelegateProfileTransactorSession) Unpause() (*types.Transaction, error) {
	return _DelegateProfile.Contract.Unpause(&_DelegateProfile.TransactOpts)
}

// UpdateProfile is a paid mutator transaction binding the contract method 0x6eeb9b10.
//
// Solidity: function updateProfile(_name string, _value bytes) returns()
func (_DelegateProfile *DelegateProfileTransactor) UpdateProfile(opts *bind.TransactOpts, _name string, _value []byte) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "updateProfile", _name, _value)
}

// UpdateProfile is a paid mutator transaction binding the contract method 0x6eeb9b10.
//
// Solidity: function updateProfile(_name string, _value bytes) returns()
func (_DelegateProfile *DelegateProfileSession) UpdateProfile(_name string, _value []byte) (*types.Transaction, error) {
	return _DelegateProfile.Contract.UpdateProfile(&_DelegateProfile.TransactOpts, _name, _value)
}

// UpdateProfile is a paid mutator transaction binding the contract method 0x6eeb9b10.
//
// Solidity: function updateProfile(_name string, _value bytes) returns()
func (_DelegateProfile *DelegateProfileTransactorSession) UpdateProfile(_name string, _value []byte) (*types.Transaction, error) {
	return _DelegateProfile.Contract.UpdateProfile(&_DelegateProfile.TransactOpts, _name, _value)
}

// UpdateProfileForDelegate is a paid mutator transaction binding the contract method 0x199baa71.
//
// Solidity: function updateProfileForDelegate(_delegate address, _name string, _value bytes) returns()
func (_DelegateProfile *DelegateProfileTransactor) UpdateProfileForDelegate(opts *bind.TransactOpts, _delegate common.Address, _name string, _value []byte) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "updateProfileForDelegate", _delegate, _name, _value)
}

// UpdateProfileForDelegate is a paid mutator transaction binding the contract method 0x199baa71.
//
// Solidity: function updateProfileForDelegate(_delegate address, _name string, _value bytes) returns()
func (_DelegateProfile *DelegateProfileSession) UpdateProfileForDelegate(_delegate common.Address, _name string, _value []byte) (*types.Transaction, error) {
	return _DelegateProfile.Contract.UpdateProfileForDelegate(&_DelegateProfile.TransactOpts, _delegate, _name, _value)
}

// UpdateProfileForDelegate is a paid mutator transaction binding the contract method 0x199baa71.
//
// Solidity: function updateProfileForDelegate(_delegate address, _name string, _value bytes) returns()
func (_DelegateProfile *DelegateProfileTransactorSession) UpdateProfileForDelegate(_delegate common.Address, _name string, _value []byte) (*types.Transaction, error) {
	return _DelegateProfile.Contract.UpdateProfileForDelegate(&_DelegateProfile.TransactOpts, _delegate, _name, _value)
}

// UpdateProfileWithByteCode is a paid mutator transaction binding the contract method 0x37d1f437.
//
// Solidity: function updateProfileWithByteCode(_byteCode bytes) returns()
func (_DelegateProfile *DelegateProfileTransactor) UpdateProfileWithByteCode(opts *bind.TransactOpts, _byteCode []byte) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "updateProfileWithByteCode", _byteCode)
}

// UpdateProfileWithByteCode is a paid mutator transaction binding the contract method 0x37d1f437.
//
// Solidity: function updateProfileWithByteCode(_byteCode bytes) returns()
func (_DelegateProfile *DelegateProfileSession) UpdateProfileWithByteCode(_byteCode []byte) (*types.Transaction, error) {
	return _DelegateProfile.Contract.UpdateProfileWithByteCode(&_DelegateProfile.TransactOpts, _byteCode)
}

// UpdateProfileWithByteCode is a paid mutator transaction binding the contract method 0x37d1f437.
//
// Solidity: function updateProfileWithByteCode(_byteCode bytes) returns()
func (_DelegateProfile *DelegateProfileTransactorSession) UpdateProfileWithByteCode(_byteCode []byte) (*types.Transaction, error) {
	return _DelegateProfile.Contract.UpdateProfileWithByteCode(&_DelegateProfile.TransactOpts, _byteCode)
}

// UpdateProfileWithByteCodeForDelegate is a paid mutator transaction binding the contract method 0xac468ebc.
//
// Solidity: function updateProfileWithByteCodeForDelegate(_delegate address, _byteCode bytes) returns()
func (_DelegateProfile *DelegateProfileTransactor) UpdateProfileWithByteCodeForDelegate(opts *bind.TransactOpts, _delegate common.Address, _byteCode []byte) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "updateProfileWithByteCodeForDelegate", _delegate, _byteCode)
}

// UpdateProfileWithByteCodeForDelegate is a paid mutator transaction binding the contract method 0xac468ebc.
//
// Solidity: function updateProfileWithByteCodeForDelegate(_delegate address, _byteCode bytes) returns()
func (_DelegateProfile *DelegateProfileSession) UpdateProfileWithByteCodeForDelegate(_delegate common.Address, _byteCode []byte) (*types.Transaction, error) {
	return _DelegateProfile.Contract.UpdateProfileWithByteCodeForDelegate(&_DelegateProfile.TransactOpts, _delegate, _byteCode)
}

// UpdateProfileWithByteCodeForDelegate is a paid mutator transaction binding the contract method 0xac468ebc.
//
// Solidity: function updateProfileWithByteCodeForDelegate(_delegate address, _byteCode bytes) returns()
func (_DelegateProfile *DelegateProfileTransactorSession) UpdateProfileWithByteCodeForDelegate(_delegate common.Address, _byteCode []byte) (*types.Transaction, error) {
	return _DelegateProfile.Contract.UpdateProfileWithByteCodeForDelegate(&_DelegateProfile.TransactOpts, _delegate, _byteCode)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_DelegateProfile *DelegateProfileTransactor) Withdraw(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _DelegateProfile.contract.Transact(opts, "withdraw")
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_DelegateProfile *DelegateProfileSession) Withdraw() (*types.Transaction, error) {
	return _DelegateProfile.Contract.Withdraw(&_DelegateProfile.TransactOpts)
}

// Withdraw is a paid mutator transaction binding the contract method 0x3ccfd60b.
//
// Solidity: function withdraw() returns()
func (_DelegateProfile *DelegateProfileTransactorSession) Withdraw() (*types.Transaction, error) {
	return _DelegateProfile.Contract.Withdraw(&_DelegateProfile.TransactOpts)
}

// DelegateProfileFeeUpdatedIterator is returned from FilterFeeUpdated and is used to iterate over the raw logs and unpacked data for FeeUpdated events raised by the DelegateProfile contract.
type DelegateProfileFeeUpdatedIterator struct {
	Event *DelegateProfileFeeUpdated // Event containing the contract specifics and raw log

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
func (it *DelegateProfileFeeUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegateProfileFeeUpdated)
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
		it.Event = new(DelegateProfileFeeUpdated)
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
func (it *DelegateProfileFeeUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegateProfileFeeUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegateProfileFeeUpdated represents a FeeUpdated event raised by the DelegateProfile contract.
type DelegateProfileFeeUpdated struct {
	Fee *big.Int
	Raw types.Log // Blockchain specific contextual infos
}

// FilterFeeUpdated is a free log retrieval operation binding the contract event 0x8c4d35e54a3f2ef1134138fd8ea3daee6a3c89e10d2665996babdf70261e2c76.
//
// Solidity: e FeeUpdated(fee uint256)
func (_DelegateProfile *DelegateProfileFilterer) FilterFeeUpdated(opts *bind.FilterOpts) (*DelegateProfileFeeUpdatedIterator, error) {

	logs, sub, err := _DelegateProfile.contract.FilterLogs(opts, "FeeUpdated")
	if err != nil {
		return nil, err
	}
	return &DelegateProfileFeeUpdatedIterator{contract: _DelegateProfile.contract, event: "FeeUpdated", logs: logs, sub: sub}, nil
}

// WatchFeeUpdated is a free log subscription operation binding the contract event 0x8c4d35e54a3f2ef1134138fd8ea3daee6a3c89e10d2665996babdf70261e2c76.
//
// Solidity: e FeeUpdated(fee uint256)
func (_DelegateProfile *DelegateProfileFilterer) WatchFeeUpdated(opts *bind.WatchOpts, sink chan<- *DelegateProfileFeeUpdated) (event.Subscription, error) {

	logs, sub, err := _DelegateProfile.contract.WatchLogs(opts, "FeeUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegateProfileFeeUpdated)
				if err := _DelegateProfile.contract.UnpackLog(event, "FeeUpdated", log); err != nil {
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

// DelegateProfileFieldDeprecatedIterator is returned from FilterFieldDeprecated and is used to iterate over the raw logs and unpacked data for FieldDeprecated events raised by the DelegateProfile contract.
type DelegateProfileFieldDeprecatedIterator struct {
	Event *DelegateProfileFieldDeprecated // Event containing the contract specifics and raw log

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
func (it *DelegateProfileFieldDeprecatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegateProfileFieldDeprecated)
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
		it.Event = new(DelegateProfileFieldDeprecated)
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
func (it *DelegateProfileFieldDeprecatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegateProfileFieldDeprecatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegateProfileFieldDeprecated represents a FieldDeprecated event raised by the DelegateProfile contract.
type DelegateProfileFieldDeprecated struct {
	Name string
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterFieldDeprecated is a free log retrieval operation binding the contract event 0xf47b35d35c737e18368ebfa5496bc97dabcea3e7b0075269da84fc32d0f201b8.
//
// Solidity: e FieldDeprecated(name string)
func (_DelegateProfile *DelegateProfileFilterer) FilterFieldDeprecated(opts *bind.FilterOpts) (*DelegateProfileFieldDeprecatedIterator, error) {

	logs, sub, err := _DelegateProfile.contract.FilterLogs(opts, "FieldDeprecated")
	if err != nil {
		return nil, err
	}
	return &DelegateProfileFieldDeprecatedIterator{contract: _DelegateProfile.contract, event: "FieldDeprecated", logs: logs, sub: sub}, nil
}

// WatchFieldDeprecated is a free log subscription operation binding the contract event 0xf47b35d35c737e18368ebfa5496bc97dabcea3e7b0075269da84fc32d0f201b8.
//
// Solidity: e FieldDeprecated(name string)
func (_DelegateProfile *DelegateProfileFilterer) WatchFieldDeprecated(opts *bind.WatchOpts, sink chan<- *DelegateProfileFieldDeprecated) (event.Subscription, error) {

	logs, sub, err := _DelegateProfile.contract.WatchLogs(opts, "FieldDeprecated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegateProfileFieldDeprecated)
				if err := _DelegateProfile.contract.UnpackLog(event, "FieldDeprecated", log); err != nil {
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

// DelegateProfileNewFieldIterator is returned from FilterNewField and is used to iterate over the raw logs and unpacked data for NewField events raised by the DelegateProfile contract.
type DelegateProfileNewFieldIterator struct {
	Event *DelegateProfileNewField // Event containing the contract specifics and raw log

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
func (it *DelegateProfileNewFieldIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegateProfileNewField)
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
		it.Event = new(DelegateProfileNewField)
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
func (it *DelegateProfileNewFieldIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegateProfileNewFieldIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegateProfileNewField represents a NewField event raised by the DelegateProfile contract.
type DelegateProfileNewField struct {
	Name string
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterNewField is a free log retrieval operation binding the contract event 0x53096991d49a1876b3be4d7f3d107f7f92043e0fceec1e81b5ba38841d78123b.
//
// Solidity: e NewField(name string)
func (_DelegateProfile *DelegateProfileFilterer) FilterNewField(opts *bind.FilterOpts) (*DelegateProfileNewFieldIterator, error) {

	logs, sub, err := _DelegateProfile.contract.FilterLogs(opts, "NewField")
	if err != nil {
		return nil, err
	}
	return &DelegateProfileNewFieldIterator{contract: _DelegateProfile.contract, event: "NewField", logs: logs, sub: sub}, nil
}

// WatchNewField is a free log subscription operation binding the contract event 0x53096991d49a1876b3be4d7f3d107f7f92043e0fceec1e81b5ba38841d78123b.
//
// Solidity: e NewField(name string)
func (_DelegateProfile *DelegateProfileFilterer) WatchNewField(opts *bind.WatchOpts, sink chan<- *DelegateProfileNewField) (event.Subscription, error) {

	logs, sub, err := _DelegateProfile.contract.WatchLogs(opts, "NewField")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegateProfileNewField)
				if err := _DelegateProfile.contract.UnpackLog(event, "NewField", log); err != nil {
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

// DelegateProfilePauseIterator is returned from FilterPause and is used to iterate over the raw logs and unpacked data for Pause events raised by the DelegateProfile contract.
type DelegateProfilePauseIterator struct {
	Event *DelegateProfilePause // Event containing the contract specifics and raw log

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
func (it *DelegateProfilePauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegateProfilePause)
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
		it.Event = new(DelegateProfilePause)
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
func (it *DelegateProfilePauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegateProfilePauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegateProfilePause represents a Pause event raised by the DelegateProfile contract.
type DelegateProfilePause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterPause is a free log retrieval operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: e Pause()
func (_DelegateProfile *DelegateProfileFilterer) FilterPause(opts *bind.FilterOpts) (*DelegateProfilePauseIterator, error) {

	logs, sub, err := _DelegateProfile.contract.FilterLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return &DelegateProfilePauseIterator{contract: _DelegateProfile.contract, event: "Pause", logs: logs, sub: sub}, nil
}

// WatchPause is a free log subscription operation binding the contract event 0x6985a02210a168e66602d3235cb6db0e70f92b3ba4d376a33c0f3d9434bff625.
//
// Solidity: e Pause()
func (_DelegateProfile *DelegateProfileFilterer) WatchPause(opts *bind.WatchOpts, sink chan<- *DelegateProfilePause) (event.Subscription, error) {

	logs, sub, err := _DelegateProfile.contract.WatchLogs(opts, "Pause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegateProfilePause)
				if err := _DelegateProfile.contract.UnpackLog(event, "Pause", log); err != nil {
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

// DelegateProfileProfileUpdatedIterator is returned from FilterProfileUpdated and is used to iterate over the raw logs and unpacked data for ProfileUpdated events raised by the DelegateProfile contract.
type DelegateProfileProfileUpdatedIterator struct {
	Event *DelegateProfileProfileUpdated // Event containing the contract specifics and raw log

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
func (it *DelegateProfileProfileUpdatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegateProfileProfileUpdated)
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
		it.Event = new(DelegateProfileProfileUpdated)
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
func (it *DelegateProfileProfileUpdatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegateProfileProfileUpdatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegateProfileProfileUpdated represents a ProfileUpdated event raised by the DelegateProfile contract.
type DelegateProfileProfileUpdated struct {
	Delegate common.Address
	Name     string
	Value    []byte
	Raw      types.Log // Blockchain specific contextual infos
}

// FilterProfileUpdated is a free log retrieval operation binding the contract event 0x217aa5ef0b78f028d51fd573433bdbe2daf6f8505e6a71f3af1393c8440b341b.
//
// Solidity: e ProfileUpdated(delegate address, name string, value bytes)
func (_DelegateProfile *DelegateProfileFilterer) FilterProfileUpdated(opts *bind.FilterOpts) (*DelegateProfileProfileUpdatedIterator, error) {

	logs, sub, err := _DelegateProfile.contract.FilterLogs(opts, "ProfileUpdated")
	if err != nil {
		return nil, err
	}
	return &DelegateProfileProfileUpdatedIterator{contract: _DelegateProfile.contract, event: "ProfileUpdated", logs: logs, sub: sub}, nil
}

// WatchProfileUpdated is a free log subscription operation binding the contract event 0x217aa5ef0b78f028d51fd573433bdbe2daf6f8505e6a71f3af1393c8440b341b.
//
// Solidity: e ProfileUpdated(delegate address, name string, value bytes)
func (_DelegateProfile *DelegateProfileFilterer) WatchProfileUpdated(opts *bind.WatchOpts, sink chan<- *DelegateProfileProfileUpdated) (event.Subscription, error) {

	logs, sub, err := _DelegateProfile.contract.WatchLogs(opts, "ProfileUpdated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegateProfileProfileUpdated)
				if err := _DelegateProfile.contract.UnpackLog(event, "ProfileUpdated", log); err != nil {
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

// DelegateProfileUnpauseIterator is returned from FilterUnpause and is used to iterate over the raw logs and unpacked data for Unpause events raised by the DelegateProfile contract.
type DelegateProfileUnpauseIterator struct {
	Event *DelegateProfileUnpause // Event containing the contract specifics and raw log

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
func (it *DelegateProfileUnpauseIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(DelegateProfileUnpause)
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
		it.Event = new(DelegateProfileUnpause)
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
func (it *DelegateProfileUnpauseIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *DelegateProfileUnpauseIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// DelegateProfileUnpause represents a Unpause event raised by the DelegateProfile contract.
type DelegateProfileUnpause struct {
	Raw types.Log // Blockchain specific contextual infos
}

// FilterUnpause is a free log retrieval operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: e Unpause()
func (_DelegateProfile *DelegateProfileFilterer) FilterUnpause(opts *bind.FilterOpts) (*DelegateProfileUnpauseIterator, error) {

	logs, sub, err := _DelegateProfile.contract.FilterLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return &DelegateProfileUnpauseIterator{contract: _DelegateProfile.contract, event: "Unpause", logs: logs, sub: sub}, nil
}

// WatchUnpause is a free log subscription operation binding the contract event 0x7805862f689e2f13df9f062ff482ad3ad112aca9e0847911ed832e158c525b33.
//
// Solidity: e Unpause()
func (_DelegateProfile *DelegateProfileFilterer) WatchUnpause(opts *bind.WatchOpts, sink chan<- *DelegateProfileUnpause) (event.Subscription, error) {

	logs, sub, err := _DelegateProfile.contract.WatchLogs(opts, "Unpause")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(DelegateProfileUnpause)
				if err := _DelegateProfile.contract.UnpackLog(event, "Unpause", log); err != nil {
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
