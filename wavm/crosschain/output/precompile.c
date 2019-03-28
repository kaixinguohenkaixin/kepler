#include "vntlib.h"



struct UserPubToCon {
    address pubAddr;
    string conAddr;
    uint256 value;
    string txid;
};


struct RollbackOfUserP2C {
    string txid;
    mapping(address, bool) agreedOrgs;
    uint32 agreedOrgsLength;
    address receiver;
    uint256 value;
};


struct UserConToPub {
    mapping(address, bool) agreedOrgs;
    uint32 agreedOrgsLength;
    address pubAddr;
    uint256 value;
    string txid;
};


KEY address emptyAddr = "";

KEY string emptyStr = "";

KEY uint32 orgsCount;

KEY uint32 testFlag;


KEY mapping(address, uint256) orgsBalance;

KEY mapping(address, address) orgsAddr;

KEY mapping(string, struct UserPubToCon) userPubToConMap;

KEY mapping(string, struct RollbackOfUserP2C) rollbackOfUserP2CMap;

KEY mapping(string, address) processedTxIDs;

KEY mapping(string, struct UserConToPub) userConToPubMap;


EVENT LogUserToC(indexed string event_name, string _ac_address, indexed address _a_address, string txid, indexed uint256 value);

EVENT LogCToUser(indexed string event_name, indexed address agreed_orgs, indexed address receiver, string txid, indexed uint256 value);

/**
  * Initializes contract
  */

void keyxo1539qc(){
AddKeyInfo( &userPubToConMap.value.conAddr, 6, &userPubToConMap, 9, false);
AddKeyInfo( &userPubToConMap.value.conAddr, 6, &userPubToConMap.key, 6, false);
AddKeyInfo( &userPubToConMap.value.conAddr, 6, &userPubToConMap.value.conAddr, 9, false);
AddKeyInfo( &userConToPubMap.value.agreedOrgs.value, 8, &userConToPubMap, 9, false);
AddKeyInfo( &userConToPubMap.value.agreedOrgs.value, 8, &userConToPubMap.key, 6, false);
AddKeyInfo( &userConToPubMap.value.agreedOrgs.value, 8, &userConToPubMap.value.agreedOrgs, 9, false);
AddKeyInfo( &userConToPubMap.value.agreedOrgs.value, 8, &userConToPubMap.value.agreedOrgs.key, 7, false);
AddKeyInfo( &userConToPubMap.value.txid, 6, &userConToPubMap, 9, false);
AddKeyInfo( &userConToPubMap.value.txid, 6, &userConToPubMap.key, 6, false);
AddKeyInfo( &userConToPubMap.value.txid, 6, &userConToPubMap.value.txid, 9, false);
AddKeyInfo( &userPubToConMap.value.value, 5, &userPubToConMap, 9, false);
AddKeyInfo( &userPubToConMap.value.value, 5, &userPubToConMap.key, 6, false);
AddKeyInfo( &userPubToConMap.value.value, 5, &userPubToConMap.value.value, 9, false);
AddKeyInfo( &userConToPubMap.value.agreedOrgsLength, 3, &userConToPubMap, 9, false);
AddKeyInfo( &userConToPubMap.value.agreedOrgsLength, 3, &userConToPubMap.key, 6, false);
AddKeyInfo( &userConToPubMap.value.agreedOrgsLength, 3, &userConToPubMap.value.agreedOrgsLength, 9, false);
AddKeyInfo( &userConToPubMap.value.pubAddr, 7, &userConToPubMap, 9, false);
AddKeyInfo( &userConToPubMap.value.pubAddr, 7, &userConToPubMap.key, 6, false);
AddKeyInfo( &userConToPubMap.value.pubAddr, 7, &userConToPubMap.value.pubAddr, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.receiver, 7, &rollbackOfUserP2CMap, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.receiver, 7, &rollbackOfUserP2CMap.key, 6, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.receiver, 7, &rollbackOfUserP2CMap.value.receiver, 9, false);
AddKeyInfo( &userPubToConMap.value.txid, 6, &userPubToConMap, 9, false);
AddKeyInfo( &userPubToConMap.value.txid, 6, &userPubToConMap.key, 6, false);
AddKeyInfo( &userPubToConMap.value.txid, 6, &userPubToConMap.value.txid, 9, false);
AddKeyInfo( &userPubToConMap.value.pubAddr, 7, &userPubToConMap, 9, false);
AddKeyInfo( &userPubToConMap.value.pubAddr, 7, &userPubToConMap.key, 6, false);
AddKeyInfo( &userPubToConMap.value.pubAddr, 7, &userPubToConMap.value.pubAddr, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.agreedOrgs.value, 8, &rollbackOfUserP2CMap, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.agreedOrgs.value, 8, &rollbackOfUserP2CMap.key, 6, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.agreedOrgs.value, 8, &rollbackOfUserP2CMap.value.agreedOrgs, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.agreedOrgs.value, 8, &rollbackOfUserP2CMap.value.agreedOrgs.key, 7, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.value, 5, &rollbackOfUserP2CMap, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.value, 5, &rollbackOfUserP2CMap.key, 6, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.value, 5, &rollbackOfUserP2CMap.value.value, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.agreedOrgsLength, 3, &rollbackOfUserP2CMap, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.agreedOrgsLength, 3, &rollbackOfUserP2CMap.key, 6, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.agreedOrgsLength, 3, &rollbackOfUserP2CMap.value.agreedOrgsLength, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.txid, 6, &rollbackOfUserP2CMap, 9, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.txid, 6, &rollbackOfUserP2CMap.key, 6, false);
AddKeyInfo( &rollbackOfUserP2CMap.value.txid, 6, &rollbackOfUserP2CMap.value.txid, 9, false);
AddKeyInfo( &orgsBalance.value, 5, &orgsBalance, 9, false);
AddKeyInfo( &orgsBalance.value, 5, &orgsBalance.key, 7, false);
AddKeyInfo( &emptyAddr, 7, &emptyAddr, 9, false);
AddKeyInfo( &orgsCount, 3, &orgsCount, 9, false);
AddKeyInfo( &processedTxIDs.value, 7, &processedTxIDs, 9, false);
AddKeyInfo( &processedTxIDs.value, 7, &processedTxIDs.key, 6, false);
AddKeyInfo( &userConToPubMap.value.value, 5, &userConToPubMap, 9, false);
AddKeyInfo( &userConToPubMap.value.value, 5, &userConToPubMap.key, 6, false);
AddKeyInfo( &userConToPubMap.value.value, 5, &userConToPubMap.value.value, 9, false);
AddKeyInfo( &testFlag, 3, &testFlag, 9, false);
AddKeyInfo( &orgsAddr.value, 7, &orgsAddr, 9, false);
AddKeyInfo( &orgsAddr.value, 7, &orgsAddr.key, 7, false);
AddKeyInfo( &emptyStr, 6, &emptyStr, 9, false);
}
constructor C() {
keyxo1539qc();
InitializeVariables();}

UNMUTABLE
uint32 GetOrgs() {
keyxo1539qc();
    return orgsCount;
}

UNMUTABLE
uint32 GetFlag() {
keyxo1539qc(); return testFlag; }

UNMUTABLE
uint256 GetOrgsBalance(address addr) {
keyxo1539qc();
    orgsBalance.key = addr;
    return orgsBalance.value;
}

/**
 * 内部函数，添加组织函数
 *
 * @param k orgsAddr和rorgsBalance的key
 * @param v orgsAddr的value，即sender
 */
void _addOrg(address k, address v) {
    orgsAddr.key = k;
    orgsAddr.value = v;
    orgsBalance.key = k;
    orgsBalance.value = U256(0);
    orgsCount += 1;
}

/**
 * 内部函数，查看组织是否已经添加
 *
 * @param addr orgsAddr的key
 */
bool _orgIsExist(address addr) {
    orgsAddr.key = addr;
    if (!Equal(orgsAddr.value, emptyAddr)) {
        return true;
    } else {
        return false;
    }
}

/**
 * 添加组织：第一次仅添加sender，以后必须由添加过的sender通过参数添加其他组织地址
 *
 * @param added_org 组织地址
 */
MUTABLE
bool AddOrg(address added_org) {
keyxo1539qc();
    address sender = GetSender();
    if (orgsCount == 0) {
        _addOrg(sender, sender);
    } else {
        
        Require(_orgIsExist(sender), "sender should be added to organization list");
        
        
        if (!_orgIsExist(added_org)) {
            _addOrg(added_org, sender);
        }
    }
    return true;
}

/**
 * 充值：组织向自己的地址充值；初始操作，因此组织地址中存有钱后无法再充值
 */
MUTABLE
bool $Charge() {
keyxo1539qc();
    address sender = GetSender();
    uint256 value = GetValue();
    
    Require(U256_Cmp(value, U256(0)) == 1, "charge amount should > 0");
    
    
    Require(_orgIsExist(sender), "sender shoud be added to the organization list");
    
    orgsBalance.key = sender;
    
    Require(U256_Cmp(orgsBalance.value, U256(0)) == 0, "organization shoud only charge once");
    
    orgsBalance.value = value;
    return true;
}

/**
 * 公链向联盟链转账
 *
 * @param _ac_address 组织在联盟链的地址
 * @param _txid 交易编号
 */
MUTABLE
bool $TransferToC(string _ac_address, string _txid) {
keyxo1539qc();
    userPubToConMap.key = _txid;
    
    Require(Equal(userPubToConMap.value.txid, emptyStr), "transaction cannot be processed twice");
    
    address sender = GetSender();
    uint256 value = GetValue();
    
    Require(U256_Cmp(value, U256(0)) == 1, "transfer amount should > 0");
   
    userPubToConMap.value.pubAddr = sender;
    userPubToConMap.value.conAddr = _ac_address;
    userPubToConMap.value.value = value;
    userPubToConMap.value.txid = _txid;
    
    PrintUint256T("Here transfer value: ", value);
    LogUserToC("LogUserToC", _ac_address, sender, _txid, value);
    return true;
}

/**
 * 内部函数，记录回滚信息
 *
 * @param txid 交易编号，作为rollbackOfUserP2CMap和userPubToConMap的key
 * @param sender 发送回退请求的组织
 * @param first 是否是第一次记录回滚信息，第一次需要录入txid、receiver和value，以后不用重复录入
 */
void _recordRollbackP2C(string txid, address sender, bool first) {
    rollbackOfUserP2CMap.key = txid;
    rollbackOfUserP2CMap.value.agreedOrgs.key = sender;
    rollbackOfUserP2CMap.value.agreedOrgs.value = true;
    rollbackOfUserP2CMap.value.agreedOrgsLength += 1;
    
    if (first) {
        rollbackOfUserP2CMap.value.txid = txid;
        userPubToConMap.key = txid;
        rollbackOfUserP2CMap.value.receiver = userPubToConMap.value.pubAddr;
        rollbackOfUserP2CMap.value.value = userPubToConMap.value.value;
    }
}

/**
 * 内部函数，删除公链向联盟链转账的记录，用于回滚成功时
 *
 * @param txid 交易编号，作为userPubToConMap的key
 */
void _delUserP2CRecord(string txid) {
    userPubToConMap.key = txid;
    userPubToConMap.value.pubAddr = emptyAddr;
    userPubToConMap.value.conAddr = emptyAddr;
    userPubToConMap.value.value = U256(0);
    userPubToConMap.value.txid = emptyStr;
}

/**
 * 公链向联盟链转账失败，回滚
 *
 * @param _txid 交易编号，作为userPubToConMap和rollbackOfUserP2CMap的key
 *
 * @return 回滚情况
 *      1： 回滚初始值；此交易在公链执行过，已经记录到rollbackOfUserP2CMap，但是并非所有组织都同意；
 *      2：
 *      3： 此交易并未在公链执行，因此回滚成功（OK）；
 *      4： 此交易在公链执行过，记录到rollbackOfUserP2CMap，此时并没有满足所有组织都同意的要求；
 *      5：
 *      6： 此交易在公链执行过，已经记录到rollbackOfUserP2CMap，并且所有组织都同意，将金额退还给用户公链账户(OK)。
 */
MUTABLE
uint32 CRollback(string _txid) {
keyxo1539qc();
    uint32 isRollback = 1;
    
    userPubToConMap.key = _txid;
    
    if (Equal(userPubToConMap.value.txid, emptyStr)) {
        isRollback = 3;
        PrintStr("CRollback no recorded txid: ", _txid);
        return isRollback;
    }
    
    address sender = GetSender();
    rollbackOfUserP2CMap.key = _txid;
    
    if (Equal(rollbackOfUserP2CMap.value.txid, emptyStr)) {
        _recordRollbackP2C(_txid, sender, true);
        isRollback = 4;
        PrintStr("CRollback record rollback txid first: ", _txid);
        return isRollback;
    } else {
        rollbackOfUserP2CMap.key = _txid;
        
        bool isFind = false;
        for (uint32 i = 0; i < rollbackOfUserP2CMap.value.agreedOrgsLength; i++) {
            rollbackOfUserP2CMap.value.agreedOrgs.key = sender;
            if (rollbackOfUserP2CMap.value.agreedOrgs.value) {
                isFind = true;
            }
        }
        if (!isFind) {
            _recordRollbackP2C(_txid, sender, false);
            PrintAddress("CRollback record rollback from address: ", sender);
        }
        
        
        if (rollbackOfUserP2CMap.value.agreedOrgsLength >= orgsCount) {
            _delUserP2CRecord(_txid);
            SendFromContract(rollbackOfUserP2CMap.value.receiver, rollbackOfUserP2CMap.value.value);
            PrintAddress("CRollback receiver: ", rollbackOfUserP2CMap.value.receiver);
            PrintUint256T("CRollback value: ", rollbackOfUserP2CMap.value.value);
            isRollback = 6;
            return isRollback;
        }
    }
    return isRollback;
}

/**
 * 内部函数，记录联盟链向公链转账信息
 *
 * @param txid 交易编号，作为userConToPubMap的key
 * @param sender 发送交易请求的组织
 * @param receiver 用户公链地址
 * @param value 交易金额
 * @param first 是否是第一次记录交易信息，第一次需要录入txid、receiver和value，以后不用重复录入
 */
void _recordUserC2PRecord(string txid, address sender, address receiver, uint256 value, bool first) {
    userConToPubMap.key = txid;
    userConToPubMap.value.agreedOrgs.key = sender;
    userConToPubMap.value.agreedOrgs.value = true;
    userConToPubMap.value.agreedOrgsLength += 1;
    
    if (first) {
        userConToPubMap.value.txid = txid;
        userConToPubMap.value.pubAddr = receiver;
        userConToPubMap.value.value = value;
    }
}

/**
 * 联盟链向公链转账
 *
 * @param _txid 交易编号，作为processedTxIDs和userConToPubMap的key
 * @param _receiver 用户公链地址
 * @param _value 交易金额
 */
MUTABLE
bool CTransfer(string _txid, address _receiver, uint256 _value) {
keyxo1539qc();
    address selfAddr = GetContractAddress();
    uint256 selfBalance = GetBalanceFromAddress(selfAddr);
    
    Require(U256_Cmp(selfBalance, _value) == 1, "transfer amount should < contract balance");
    
    processedTxIDs.key = _txid;
    
    if (!Equal(processedTxIDs.value, emptyAddr)) {
        PrintStr("CTransfer processed: ", _txid);
        return true;
    }
    
    address sender = GetSender();
    userConToPubMap.key = _txid;
    
    if (Equal(userConToPubMap.value.txid, emptyStr)) {
        _recordUserC2PRecord(_txid, sender, _receiver, _value, true);
        PrintStr("CTransfer record first: ", _txid);
    } else {
        
        if (!Equal(userConToPubMap.value.pubAddr, _receiver)) {
            PrintAddress("CTransfer notEqual Addr1: ", userConToPubMap.value.pubAddr);
            PrintAddress("CTransfer notEqual Addr2: ", _receiver);
            return false;
        }
        
        
        if (U256_Cmp(userConToPubMap.value.value, _value) != 0) {
            PrintUint256T("CTransfer value not the same: ", _value);
            return false;
        }
        
        
        bool isFind = false;
        for (uint32 i = 0; i < userConToPubMap.value.agreedOrgsLength; i++) {
            userConToPubMap.value.agreedOrgs.key = sender;
            if (userConToPubMap.value.agreedOrgs.value) {
                isFind = true;
            }
        }
        if (!isFind) {
            _recordUserC2PRecord(_txid, sender, _receiver, _value, false);
            PrintAddress("CTransfer record: ", _receiver);
        }
        
        
        if (userConToPubMap.value.agreedOrgsLength >= orgsCount) {
            SendFromContract(_receiver, _value);
            PrintAddress("CTransfer receiver: ", _receiver);
            PrintUint256T("CTransfer value: ", _value);
            processedTxIDs.key = _txid;
            processedTxIDs.value = _receiver;
        }
    }
    
    LogCToUser("LogCToUser", sender, _receiver, _txid, _value);
    return true;
}
