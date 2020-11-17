# posa

该文档对posa共识算法及系统合约流程进行了详细的说明。

## 共识算法介绍

posa算法基于clique算法修改而来，增加了系统合约实现了Validator质押/准入的相关功能，可以对Validator进行激励(按质押比例获得手续费)和惩罚(没收当前收益、移除出validator列表)。

系统功能(golang共识代码功能): 

1. 在第一个块时，初始化系统合约

2. 在块周期结束时(`number%Epoch==0`)时:
    * 通过系统合约`getTopValidators`获取当前的排名靠前的TopValidators，并将其填入extraData字段
    * 调用系统合约`updateActiveValidatorSet`更新合约当前激活的Validators列表
    * 调用系统合约`decreaseMissedBlocksCounter`尝试削减validator的出错次数，避免因出错次数一直累加而被意外移除出validator列表

3. 生产块周期第一个块时更换为新的validator列表，此时新的validator可以出块，合约中validator列表变化必须在下一块周期才会生效。

4. 当有out of turn的块出现时，且本应出块的validator最近未出块，则validator调用系统合约`punish`接口对validator进行惩罚。如果validator出错次数达到punishThreshold(默认10)，则会没收当前收益。当达到removeThreshold(默认30)时，则踢出validator列表，状态设置为Jailed。


合约功能:

1. Validator合约：
    * 通过了proposal的用户成为validator
    * 任意人可以对validator stake
    * staker可以赎回质押的hb
    * validator设置的feeAddr可以赎回收益

2. Proposal合约：
    * 用户创建提案，申请成为validator
    * validator对提案进行投票

3. Punish合约: 主要由系统进行调用，惩罚miss block的validator

## 用户成为validator流程

1. 调用Proposal合约`createProposal`接口创建提案，提案ID在事件中可以得到

2. Validator对该合约进行投票

3. 投票通过后，用户调用Validators合约`createOrEditValidator`接口，创建validator。

4. 任意用户可以对validator进行stake操作，当validator 总押金排名在前21时，则其进入top validators列表。

4. 系统在block epoch时，调用Validators合约获取top validator list，并将其写入extra data，并在下一个块更新validator列表，此时新validator可以出块。

## 参数配置

共识参数主要分为两个部分:

1. 出块时间、块周期等内容，可在创世块中配置。

2. 合约参数配置，需要修改合约源代码，并其在创世块中设置对应系统合约的源代码。

### 创世块配置

以下内容必须在创世块中进行配置:

- Period: 出块时间间隔，设置为0则表示只有在有交易时才出块

- Epoch: 更新validators的块数间隔，系统在块周期第一个块更新validators列表

- 账户设置，请在创世块中设置系统合约相应的代码(**deployedCode**):
    - Validators(0x000000000000000000000000000000000000f000): 验证者合约
    - Punish(0x000000000000000000000000000000000000f001): 惩罚合约
    - Proposal(0x000000000000000000000000000000000000f002): 提案合约

### 系统合约参数

通用参数：

- MaxValidators(21): 默认最大激活的validators个数

- StakingLockPeriod(100): validator申请赎回质押hb操作后到实际可以赎回质押hb的块间隔。

- WithdrawProfitPeriod(100): validators连续赎回收益之间最小的间隔块大小。系统slash validator时，会将其当前未赎回的收益清零，均分给其他的validators。

- MinimalStakingCoin(32 ether): 成为validator候选的最小质押hb数量


Punish合约：

- punishThreshold(10): 没收当前收益出错阀值；当validator掉线导致未出块次数达到该阀值后，将会没收该validator的当前收益，均分给其他的validators

- removeThreshold(30): 移除validator列表出错阀值；当validator掉线导致未出块次数达到该阀值后，将会没收其当前收益，且将其移除出validator列表

- decreaseRate(4): validator出错清除比例。每经过一个epoch时，系统会自动发送系统交易，对validator的出错数量按一定比例进行削减(防止出错比例很小，但值一直累加从而产生惩罚)。削减规则: 如果validator出错次未超过removeThreshold / decreaseRate，不进行任何操作；否则将其出错次数削减removeThreshold / decreaseRate


Proposal合约：

- proposalLastingPeriod(7 days): 提案存在时间，当超出该时间段后，提案没有通过的话则作废

## 系统合约用户接口

本部分介绍了可供外部调用的接口.

### Proposal

用户如果想成为validator，必须由自己或者其他人创建提案，申请成为validator。当前激活的validator可以对该提案进行投票，当同意的人数超过了1半时，则提案通过(永久有效)，用户之后可以通过质押hb的方式成为validator候选。

#### createProposal

任意用户创建提案(提案默认持续7天，7天内提案没有通过则需要重新创建提案)。

```solidity
# dst: 成为validator候选人的地址
# details: validator候选人的详细说明(可选, 长度应不大于3000)
createProposal(address dst, string calldata details)


# 交易产生的日志
# id: 提案id，可用于投票
# proposer: 提案人
# dst: validator候选地址
# time: 提案时间
event LogCreateProposal(
    bytes32 indexed id,
    address indexed proposer,
    address indexed dst,
    uint256 time
);
```

#### voteProposal

当前validator对提案进行投票。当同意票数超过半数时，则提案通过

```solidity
# id: 提案id
# auth: 是否同意该提案
voteProposal(bytes32 id, bool auth)


# 交易产生日志
# 提案通过日志
# id: 提案id
# dst: validator候选地址
# time: 通过时间
event LogPassProposal(
    bytes32 indexed id,
    address indexed dst,
    uint256 time
);
# 提案未通过日志(超过半数不同意)
# id: 提案id
# dst: validator候选地址
# time: 提案未通过时间
event LogRejectProposal(
    bytes32 indexed id,
    address indexed dst,
    uint256 time
);
```

### Validators

validator调用该合约进行质押、赎回押金、赎回收益等相关操作.

管理员调用接口:

validator调用接口:

```solidity
# 用户向validator质押押金
stake(address validator)

# 交易日志
# 第一次成为validator
# staker: 质押的用户
# val: validator地址
# staking: 质押的hb数量
# time: 交易时间
event LogStake(
    address indexed staker,
    address indexed val,
    uint256 staking,
    uint256 time
);

# 成为top validator
# val: validator地址
# time: 交易时间
event LogAddToTopValidators(address indexed val, uint256 time);


# 编辑validator信息
# feeAddr: 受益地址
# moniker: 名称，长度不大于70
# identity: 身份信息，长度不大于3000
# website: 网站信息，长度不大于140
# email: 邮件信息，长度不大于140
# details: 详细信息，长度不大于280
createOrEditValidator(
    address payable feeAddr,
    string calldata moniker,
    string calldata identity,
    string calldata website,
    string calldata email,
    string calldata details
)

# 交易日志
# 第一次创建validator信息
event LogCreateValidator(
    address indexed val,
    address indexed fee,
    uint256 time
);
# 编辑validator信息
# val: validator地址
# fee: 更新后的受益地址
# time: 更新时间
event LogEditValidator(
    address indexed val,
    address indexed fee,
    uint256 time
);



# 质押的用户申请赎回资金(当对应的validator总质押的金额小于设置的最小值时，其自动被系统移除列表，且需要重新create proposal才能再次成为validator)
# 注意：押金不会马上发送给用户，需要经过系统设定的时间(100个块)后调用withdrawStaking()才可以赎回押金
unstake(address validator)
# 交易日志
# staker: 赎回押金的用户
# val: validator地址
# amount: 押金金额
# time: 交易时间
event LogUnstake(
    address indexed staker,
    address indexed val,
    uint256 amount,
    uint256 time
);



# 赎回押金
withdrawStaking(address validator)
# 交易日志
# staker: 赎回押金的用户
# val: validator地址
# amount: 押金金额
# time: 交易时间
event LogWithdrawStaking(
    address indexed staker,
    address indexed val,
    uint256 amount,
    uint256 time
);



# 赎回出块收益，该方法由收益地址调用，调用时需传入赎回的是哪个validator的收益
# validator: validator地址
withdrawProfits(address validator)
# 交易事件
# val: validator的地址
# fee: 收益人的地址
# hb: hb收益金额
# time: 交易时间
event LogWithdrawProfits(
    address indexed val,
    address indexed fee,
    uint256 hb,
    uint256 time
);



# 查看当前激活的validator列表，当前可以生产块的validator列表
getActiveValidators() returns (address[] memory)
# 查看当前top validator列表，当前质押金额最高的validator列表，下一周期会被激活
getTopValidators() returns (address[] memory)
# 返回validator基本信息
getValidatorDescription(address val) returns (
    string memory moniker,
    string memory identity,
    string memory website,
    string memory email,
    string memory details
)
# 返回validator详细信息
getValidatorInfo(address val)return (
    feeAddr, // 收益地址
    status, // 当前状态
    coins, // 总押金
    hbIncoming, // hb收益
    totalJailedHB, // 被没收的hb
    lastWithdrawProfitsBlock, // 上次赎回收益的块号
    stakers // 向该validator质押的用户列表
);
# 返回质押的详细信息
getStakingInfo(address staker, address val) return (
    coins, // 押金金额
    unstakeBlock, // 执行unstake操作的块号
    index // 在validator的stakers列表中是索引
);
```
