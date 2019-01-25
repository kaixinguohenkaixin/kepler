## KEPLER

Kepler是VNT Chain的重要组成部分，是Hubble和Galileo 之间的桥梁，是实现价值互联的关键。Kepler基于公证人机制，支持资产的跨链流转与信息的跨链交互，帮助各业务场景进行连接和扩展，提高资产流动性的同时降低交易成本，进而构建一个高效、低成本的聚合价值网络。

Kepler跨链系统——支持跨链资产交换及转移，实现不同区块链业务场景的信息交互，帮助区块链之间进行连接和扩展。它通过与智能合约的交互，采用事件监听机制，保障了资产在不同区块链之间的流动性。基于Kepler跨链系统，用户既可以维持联盟链原有的数据隐私和授权使用的特性，也可以获得Hubble Network的token对联盟链业务的结算能力。

## 特点
1. 采用事件监听机制，实现跨链价值的迅速安全转移。
2. 采用重试和超时检验策略，保证交易失效的回滚。
3. 通过与智能合约的交互，实现对用户账户和跨链交易的监控管理。


## 从源码安装kepler

安装`kepler`需要Go编译器（版本大于1.11）和Docker（版本大于18.09）。

首先，克隆仓库`kepler`到路径`$GOPATH/src/github.com/vntchain`，并进入项目目录:

    mkdir -p $GOPATH/src/github.com/vntchain
    cd $GOPATH/src/github.com/vntchain
    git clone https://github.com/vntchain/kepler
    cd kepler

然后，使用下面命令编译`kepler`:

    make kepler

或者使用下面命令编译`kepler`并生成docker镜像（建议）:

    make all

经过以上可以在`$GOPATH/src/github.com/vntchain/kepler/build/bin/`目录调用`kepler`，或者使用kepler docker镜像。

## 运行kepler

你可以在本地搭建跨链测试节点，需要根据已经部署好的Hubble和Galileo环境对config中的文件进行配置，并分别在Hubble和Galileo环境上部署跨链交易合约，参考资料在准备中。

## 资源

1. [VNT Chain官网](http://vntchain.io/)
2. [VNT Chain开发者文档](https://github.com/vntchain/vnt-documentation)
3. [VNT Chain白皮书](https://github.com/vntchain/vnt-documentation/blob/master/VNT-white-paper-CH.pdf)


## 贡献源码

欢迎PR，感谢您为`kepler`做的任何一点改进。您可以fork项目到个人仓库后、修复问题进行提交，然后向`kepler`仓库发起PR。

贡献代码请遵循以下规则，方便`kepler`核心开发人员对代码进行Review。

1. 所有代码经gofmt进行格式化。
2. PR请遵循以下规则：
    1. 标题格式：`[fixed/style/test] #Issue PR标题`，`fixed/style/test`代表了修复/调整格式/修改测试，`#Issue`为本PR相关的Issue编号，PR标题为一句简洁话，描述本次PR的目的。
    1. PR内容：描述本次PR具体的内容，希望能尽可能详细，这样能让Review本PR的开发人员了解你的意图。


## 许可证

所有`kepler`仓库生成的二进制程序都采用Apache v2.0许可证。

