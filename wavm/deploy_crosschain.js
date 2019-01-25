var Vnt = require("vnt")
var fs = require("fs")


var codeFile = "./output/precompile.wasm"
var abiFile = "./output/abi.json"
var wasmcode = fs.readFileSync(codeFile)
var wasmabi = fs.readFileSync(abiFile)
var abi = JSON.parse(wasmabi.toString("utf-8"))
var code = wasmcode.toString("base64")

var vnt = new Vnt();
vnt.setProvider(new vnt.providers.HttpProvider("http://192.168.9.33:8545"));

var coinbase = "0xe7091023bc18e490c8e8b8d1a0d8e3429f2cd5ca"
var passwd = "123456"
var org2 = "0xda59228609f8d248b088342cc4d8225038a73e31"
var org3 = "0xe50775707b7c6dd963da6862995aeb0ea848cb3c"
var org4 = "0x4023adfc1937667d8b664171e7d55153bd69d900"

contractAddr = "0x2d07598b12f1d0293f2798fd8555f40aa71ca3cd"

function DeployWasmContract() {
  var pack = {
    Code: code,
    Abi: wasmabi.toString("base64")
  }

  var json = JSON.stringify(pack)
  var txData = vnt.toHex(json)

  doDeploy(txData)
}

function doDeploy(txData) {
    vnt.personal.unlockAccount(coinbase, passwd, 250000000)
    var contract = vnt.core.contract(abi)

    var contractReturned = contract.new({
        from: coinbase,
        data: txData,
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


function AddOrg(address) {
    vnt.personal.unlockAccount(coinbase, passwd, 250000000)
    var contract = vnt.core.contract(abi).at(contractAddr)
    contract.AddOrg.sendTransaction(
    address, {from: coinbase}, function(err, txid) {
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
    conAddr, txid, {from: pubAddr, value: vnt.toWei(value.toString(), "vnt")}, function(err, txid) {
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
    txid, pubAddr, vnt.toWei(value.toString(), "vnt"), {from: sender}, function(err, txid) {
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
    txid, {from: addr}, function(err, txid) {
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
    var filter = vnt.core.filter({fromBlock:'13215', address:contractAddr})
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

// sendEther(coinbase, passwd, org2, 10)
// DeployWasmContract()

// AddOrg(coinbase)
// AddOrg(org2)
// GetOrgs()
/*
AddOrg(org3)
AddOrg(org4)
*/

// getBalance(coinbase)
// getBalance(contractAddr)
// Charge(coinbase, 10)
// getBalance(coinbase)
// GetOrgsBalance(coinbase)
// getBalance(contractAddr)
// getBalance(org2)
// Charge(org2, 1)
// GetOrgsBalance(org2)
// getBalance(org2)
// getBalance(contractAddr)

// TransferToC(org2, "1a5d0d5e8c567463b32c773f4c971e9fabbcdef8bc9507bc64f19e674def075e", "txidp2c001", 1)
// getBalance(org2)
// getBalance(contractAddr)

// CRollback(coinbase, "txidp2c001")
// CRollback(org2, "txidp2c001")
// getBalance(org2)
// getBalance(contractAddr)

// getBalance(org3)
// getBalance(contractAddr)
// CTransfer(coinbase, org3, "txidc2p001", 1)
// CTransfer(org2, org3, "txidc2p001", 1)
// getBalance(org3)
// getBalance(contractAddr)

//filterEvent()