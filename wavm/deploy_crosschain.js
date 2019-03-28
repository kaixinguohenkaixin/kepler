var Vnt = require("vnt")
var fs = require("fs")


var codeFile = "./output/C.compress"
var abiFile = "./output/abi.json"
var wasmabi = fs.readFileSync(abiFile)
var abi = JSON.parse(wasmabi.toString("utf-8"))

var vnt = new Vnt();
vnt.setProvider(new vnt.providers.HttpProvider("http://192.168.9.33:8545"));

var coinbase = "0xe7091023bc18e490c8e8b8d1a0d8e3429f2cd5ca"
var passwd = "123456"
var org1 = "0xda59228609f8d248b088342cc4d8225038a73e31"
var org2 = "0xe50775707b7c6dd963da6862995aeb0ea848cb3c"
var user1 = "0x4023adfc1937667d8b664171e7d55153bd69d900"

contractAddr = "0x0410867af739af88e6db5e95d10e1685cd3ddcd5"


function DeployWasmContract() {
    var contract = vnt.core.contract(abi).codeFile(codeFile)

	var contractReturned = contract.new({
        from: coinbase,  
        data: contract.code, 
        gas: 4000000
    }, function(err, myContract){
       if(!err) {
          if(!myContract.address) {
              console.log("transactionHash: ", myContract.transactionHash)
          } else {
              console.log("contract address: ", myContract.address)
          }
       }
     });
}

function getTransactionReceipt(tx, cb) {
  var receipt = vnt.core.getTransactionReceipt(tx)
  if(!receipt) {
      setTimeout(function () {
          getTransactionReceipt(tx, cb)
      }, 1000);
  } else {
      cb(receipt)
  }
}


function AddOrg(first, addr) {
    vnt.personal.unlockAccount(first, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    contract.AddOrg.sendTransaction(
    addr, {from: first}, function(err, txid) {
        if(err) {
            console.log("error happend: ", err)
        } else {
            console.log("txid: ", txid)
            getTransactionReceipt(txid, function(receipt) {
                console.log("receipt: ", receipt)
            })
        }
    })
}

function GetOrgs() {
    vnt.personal.unlockAccount(coinbase, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    var r = contract.GetOrgs.call({from: coinbase})
    console.log("result: ", r.toString())
}

function GetOrgsBalance(addr) {
    vnt.personal.unlockAccount(addr, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    var r = contract.GetOrgsBalance.call(addr, {from: coinbase})
    console.log("result: ", r.toString())
}

function Charge(addr, value) {
    vnt.personal.unlockAccount(addr, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    contract.$Charge.sendTransaction(
    {from: addr, value: vnt.toWei(value.toString(), "vnt")}, function(err, txid) {
        if(err) {
            console.log("error happend: ", err)
        } else {
            console.log("txid: ", txid)
            getTransactionReceipt(txid, function(receipt) {
                console.log("receipt: ", receipt)
            })
        }
    })
}

function getLogUserToCEvent() {
    vnt.personal.unlockAccount(coinbase, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    var event = contract.LogUserToC(function(error, result){
        if (!error)
            console.log(result);
    });
}

function TransferToC(pubAddr, conAddr, txid, value) {
    vnt.personal.unlockAccount(pubAddr, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    contract.$TransferToC.sendTransaction(
    conAddr, txid, {from: pubAddr, gas: 300000, value: vnt.toWei(value.toString(), "vnt")}, function(err, txid) {
        if(err) {
            console.log("error happend: ", err)
        } else {
            console.log("txid: ", txid)
            //getLogUserToCEvent()
            getTransactionReceipt(txid, function(receipt) {
                console.log("receipt: ", receipt)
            })
        }
    })
}

function getLogCToUserEvent() {
    vnt.personal.unlockAccount(coinbase, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    var event = contract.LogCToUser(function(error, result){
        if (!error)
            console.log(result);
    });
}

function CTransfer(sender, pubAddr, txid, value) {
    vnt.personal.unlockAccount(sender, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    contract.CTransfer.sendTransaction(
    txid, pubAddr, vnt.toWei(value.toString(), "vnt"), {from: sender, gas: 300000}, function(err, txid) {
        if(err) {
            console.log("error happend: ", err)
        } else {
            console.log("txid: ", txid)
            //getLogCToUserEvent()
            getTransactionReceipt(txid, function(receipt) {
                console.log("receipt: ", receipt)
            })
        }
    })
}

function CRollback(addr, txid) {
    vnt.personal.unlockAccount(addr, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    contract.CRollback.sendTransaction(
    txid, {from: addr, gas: 300000}, function(err, txid) {
        if(err) {
            console.log("error happend: ", err)
        } else {
            console.log("txid: ", txid)
            getTransactionReceipt(txid, function(receipt) {
                console.log("receipt: ", receipt)
            })
        }
    })
}

function filterEvent() {
    var filter = vnt.core.filter({fromBlock:'10', address:contractAddr})
    var myResults = filter.get(function (error, log) {
        if(error) {
            console.log("error happend: ", err)
        } else {
            console.log("Log:", log)
        }
    });
}

function getBalance(address) {
    var balance = vnt.core.getBalance(address);
    console.log("Balance: ", balance.toString());
}

function sendEther(from, password, to, value) {
    vnt.personal.unlockAccount(from, password, 250000000);
    vnt.core.sendTransaction({from: from, to: to, value: vnt.toWei(value.toString(), "vnt")}, function(err, transactionHash) {
        if (!err) {
            console.log(transactionHash); 
        }
    });
}

// ------  充钱  ------
// getBalance(coinbase)
// getBalance(org1)
// getBalance(org2)

// sendEther(coinbase, passwd, org1, 10)
// sendEther(coinbase, passwd, org2, 10)


// ------  部署合约  ------
// DeployWasmContract()


// ------  初始化合约  ------
// AddOrg(org1, org1)
// AddOrg(org1, org2)

// GetOrgs()


// ------  向total账户充值  ------
// getBalance(org1)
// getBalance(contractAddr)

// Charge(org1, 4)

// getBalance(org2)
// getBalance(contractAddr)

// Charge(org2, 1)


// ------  向联盟链转账  ------
getBalance(user1)
getBalance(contractAddr)

// sendEther(coinbase, passwd, user1, 10)

// TransferToC(user1, "1a5d0d5e8c567463b32c773f4c971e9fabbcdef8bc9507bc64f19e674def075e", "txidp2c002", 5)


// CRollback(org1, "txidp2c001")
// CRollback(org2, "txidp2c001")

// getBalance(user1)
// getBalance(contractAddr)
// CTransfer(org1, user1, "txidc2p001", 1)
// CTransfer(org2, user1, "txidc2p001", 1)

//filterEvent()
