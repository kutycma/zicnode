# ZicNode
A ZicBoard backend based on modified xray-core.
一个基于修改版xray内核的ZicBoard节点服务端。

**注意： 本项目需要搭配[ZicBoard](https://github.com/ZicBoard/ZicBoard)**

## 软件安装

### 一键安装

```
wget -N https://raw.githubusercontent.com/ZicBoard/ZicNode/master/script/install.sh && bash install.sh
```

## 构建
``` bash
GOEXPERIMENT=jsonv2 go build -v -o build_assets/zicnode -trimpath -ldflags "-X 'github.com/ZicBoard/ZicNode/cmd.version=$version' -s -w -buildid="
```

## Stars 增长记录

[![Stargazers over time](https://starchart.cc/ZicBoard/ZicNode.svg?variant=adaptive)](https://starchart.cc/ZicBoard/ZicNode)
