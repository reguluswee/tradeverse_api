# Tradeverse API

Tradeverse API 是一个基于 Go 语言开发的区块链游戏平台后端服务，提供用户认证、游戏会话管理、资产交易、链上交易处理等核心功能。

## 项目架构

```
tradeverse_api/
├── api/              # API 接口层
│   ├── http/         # HTTP 控制器
│   ├── interceptor/  # 拦截器/中间件
│   ├── oauth/        # OAuth2 认证服务
│   ├── router.go     # 路由配置
│   └── service/      # API 服务层
├── chain/            # 区块链交互层
├── config/           # 配置管理
├── model/            # 数据模型
├── service/          # 业务逻辑层
├── system/           # 系统基础组件
├── tools/            # 工具类
├── utils/            # 工具函数
└── main.go           # 程序入口
```

### 核心组件

1. **API 层** - 提供 RESTful API 接口
   - 用户认证与授权 (OAuth2/JWT)
   - 游戏会话管理
   - 资产交易处理
   - 用户信息管理

2. **区块链交互层** - 处理与区块链的交互
   - EVM 兼容链交易解析
   - 链上事件监听与处理
   - 交易队列管理

3. **数据访问层** - 数据库操作
   - MySQL 数据库访问
   - ORM 映射 (GORM)
   - 数据模型定义

4. **业务逻辑层** - 核心业务处理
   - 用户账户管理
   - 游戏会话处理
   - 资产交易处理

## 功能特性

### 用户系统
- 多种登录方式：钱包签名登录、开发者登录
- 用户资料管理
- 邮箱绑定验证
- 钱包地址绑定

### 游戏系统
- 游戏会话管理（开始、结束、确认）
- 游戏赛季系统
- 排行榜功能
- 游戏设置管理

### 资产交易系统
- 链上充值处理
- 提现申请与处理
- 资产冻结与解冻
- 交易记录查询
- 余额管理

### 区块链集成
- EVM 兼容链支持
- 智能合约交互
- 链上交易监控
- 事件监听与处理

## 安装部署

### 环境要求
- Go 1.19+
- MySQL 5.7+
- 区块链 RPC 节点

### 配置文件
创建 `config/dev.yml` 配置文件：

```yaml
database:
  type: mysql
  host: localhost
  port: 3306
  user: username
  password: password
  dbname: tradeverse
  sslmode: disable
  TimeZone: Asia/Shanghai

chain:
  - name: BSC
    wsRpc: wss://bsc-ws.example.com
    queryRpc:
      - https://bsc-rpc.example.com||100
    slotParallel: 1
    txDetal: 0
    rangeRound: 100

log:
  path: ./logs

http:
  port: 8080

contract:
  nAddress: "0x..." # N 代币合约地址
```

### 环境变量
在 `.env` 文件中配置：

```env
ENV=dev
DALINK_GO_CONFIG_PATH=./config/dev.yml
TOPUP_CONTRACT=0x... # 充值合约地址
WITHDRAW_LOCK_PK=... # 提现私钥
```

### 构建与运行
```bash
# 构建
go build -o tradeverse_api

# 运行
./tradeverse_api
```

## API 接口

### 认证接口
- `POST /spwapi/preauth/get_msg` - 获取签名消息
- `POST /spwapi/preauth/verify_msg` - 验证签名
- `POST /spwapi/preauth/register` - 用户注册
- `POST /spwapi/preauth/login` - 用户登录

### 用户接口
- `GET /spwapi/auth/user/profile` - 获取用户资料
- `POST /spwapi/auth/user/profile` - 更新用户资料
- `GET /spwapi/auth/user/balance` - 查询用户余额
- `GET /spwapi/auth/user/transactions` - 查询交易记录

### 游戏接口
- `POST /spwapi/auth/season/join` - 加入赛季
- `POST /spwapi/auth/user/game/session` - 游戏会话管理
- `GET /spwapi/auth/user/game/session` - 查询游戏会话

### 资产接口
- `POST /spwapi/auth/account/balance/topup` - 充值上报
- `GET /spwapi/auth/account/balance/topup` - 充值记录查询
- `POST /spwapi/auth/account/balance/withdraw/request` - 提现申请
- `GET /spwapi/auth/account/balance/withdraw/check` - 提现状态查询

### 开发者接口
- `POST /spwapi/devauth/game/update` - 更新游戏信息
- `POST /spwapi/devauth/game/setting/save` - 保存游戏设置
- `POST /spwapi/devauth/game/session/list` - 查询游戏会话列表

## 配置说明

### 数据库配置
```yaml
database:
  type: mysql          # 数据库类型
  host: localhost      # 数据库主机
  port: 3306           # 数据库端口
  user: username       # 数据库用户
  password: password   # 数据库密码
  dbname: tradeverse   # 数据库名
  sslmode: disable     # SSL 模式
  TimeZone: Asia/Shanghai # 时区
```

### 区块链配置
```yaml
chain:
  - name: BSC                    # 链名称
    wsRpc: wss://bsc-ws.example.com  # WebSocket RPC
    queryRpc:                    # 查询 RPC 列表
      - https://bsc-rpc.example.com||100
    slotParallel: 1            # 并行处理槽位数
    txDetal: 0                 # 交易延迟
    rangeRound: 100            # 范围轮次
```

### HTTP 配置
```yaml
http:
  port: 8080  # HTTP 服务端口
```

### 合约配置
```yaml
contract:
  nAddress: "0x..."  # N 代币合约地址
```

## 核心代码介绍

### main.go - 程序入口
程序启动文件，负责初始化配置、启动 HTTP 服务和区块链监听器。

### chain/evm_tx_q.go - 链上交易队列
处理链上交易的队列管理，包括交易入队、消费者处理等。

### api/oauth/handler.go - OAuth 认证处理
实现 OAuth2 认证流程，包括授权码模式和 JWT 令牌生成。

### api/http/controller/auth/user.go - 用户控制器
处理用户相关的 API 请求，包括资料管理、余额查询、交易处理等。

### service/account_balance.go - 账户余额服务
处理账户余额的更新逻辑，包括充值、提现、冻结等操作。

### model/ - 数据模型
定义了项目中使用的所有数据模型，包括用户、游戏、交易等相关表结构。

## 许可证

MIT License

Copyright (c) 2025 Tradeverse

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.