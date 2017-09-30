// Shelly

package tests

import (
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

func setupEnv(address string, caller string) *vm.EVM {
	pre := make(map[string]Account)
	db, _ := ethdb.NewMemDatabase()
	statedb := makePreState(db, pre)

	env := make(map[string]string)
	env["currentCoinbase"] = "0"
	env["currentDifficulty"] = "0"
	env["currentGasLimit"] = "0"
	env["currentNumber"] = "0"
	env["previousHash"] = "0"
	env["currentTimestamp"] = "0"

	exec := make(map[string]string)
	exec["address"] = address
	exec["caller"] = caller
	exec["data"] = "0"
	exec["gas"] = "0"
	exec["value"] = "0"

	chainConfig := &params.ChainConfig{
		HomesteadBlock: params.MainNetHomesteadBlock,
		DAOForkBlock:   params.MainNetDAOForkBlock,
		DAOForkSupport: true,
	}

	environment, _ := NewEVMEnvironment(true, chainConfig, statedb, env, exec)
	return environment

}

/************** UNIT TESTS ********************/
func Test1(t *testing.T) {
	// init dummy checker, evm, and contracts A,B,C.
	checker := vm.TheChecker()
	environment := setupEnv("0", "0")
	A := environment.StateDB.CreateAccount(common.StringToAddress("111111191324e6712a591f304b4eedef6ad9bb9d"))
	B := environment.StateDB.CreateAccount(common.StringToAddress("222222291324e6712a591f304b4eedef6ad9bb9d"))
	C := environment.StateDB.CreateAccount(common.StringToAddress("333333391324e6712a591f304b4eedef6ad9bb9d"))

	cA := vm.NewContract(A, A, nil, nil)
	cB := vm.NewContract(B, B, nil, nil)
	cC := vm.NewContract(C, C, nil, nil)
	fmt.Printf("Working on checker %v, environment %v, calling contract %v, %v, %v", checker, environment, cA, cB, cC)

	// Simulate: Start A, call B, call A, return to B, return to A: A1 B1 A'1 B2 A2 -> Test each of the 4 cutpoints + no cutpoint
	checker.UponEVMStart(environment.Interpreter(), cA)
	checker.UponSLoad(environment, cA, common.HexToHash("3ac225168df54212a25c1c01fd35bebfea408fdac2e31ddd6f80a4bbf9a5f1ca"), nil)
	checker.UponEVMEnd(environment.Interpreter(), cA)

	// Simulate: Start A, call B, call C, call A, return to C, return to B, return to A: A1 B1 C1 A'1 C2 B2 A2 -> Test each of the 4 cutpoints + no cutpoint

	// Simulate: Start A, call B, call A, call C, call A, return to C, return to A, return to B, return to A: A1 B1 A'1 C1 A''1 C2 A'2 B2 A2 -> Test each of the 4 inner cutpoints and 4 outer cutpoints + no cutpoint

	// A1 B1 A'1 B2 A''1 B3 A2 -> Test each of the 5 cutpoints + no cutpoint

	//

}
