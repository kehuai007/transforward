# TransForward

高性能流量转发服务，支持 TCP/UDP/TCP+UDP 协议，带 Web 管理界面。

## 功能特性

- 支持 TCP、UDP、TCP+UDP 三种转发模式
- Web 管理界面，实时状态监控
- WebSocket 实时推送连接状态和流量统计
- 密码认证，Bearer Token 登录
- 优雅退出机制
- 跨平台支持 (Windows/Linux/macOS)
- 服务化部署，开机自启
- 端口冲突检测
- 规则编辑热更新（无需重启）

## 快速开始

### 构建

```bash
# Windows
go build -o transforwardd.exe .

# Linux/macOS
go build -o transforwardd .
```

### 运行

```bash
# 直接运行（默认端口 8081）
./transforwardd

# 指定自定义端口（会保存到配置文件）
./transforwardd -port=8080

# 其他参数
./transforwardd -reset    # 重置密码
./transforwardd -version # 查看版本
```

### 服务安装

```bash
# Windows
transforwardd.exe -port=8080 -install  # 指定端口并安装服务
transforwardd.exe -start               # 启动服务
transforwardd.exe -stop                # 停止服务
transforwardd.exe -uninstall           # 卸载服务

# Linux (systemd)
sudo ./transforwardd -install
sudo ./transforwardd -start
sudo ./transforwardd -stop
sudo ./transforwardd -uninstall
```

## Web 管理

访问 `http://localhost:8081`（或自定义端口），首次使用需要设置密码。

### 功能页面

- **Dashboard** - 总览连接数、流量统计
- **Rules** - 添加/编辑/删除转发规则（添加后需手动启动）
- **Settings** - 修改密码

### 规则操作说明

| 操作 | 说明 |
|------|------|
| 添加规则 | 创建规则（处于停止状态） |
| 启动 | 开始监听端口 |
| 停止 | 停止监听 |
| 编辑 | 修改规则参数（运行中会自动重启） |
| 删除 | 停止并删除规则 |

### API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/login | 登录 |
| GET | /api/rules | 获取规则列表 |
| POST | /api/rules | 添加规则 |
| PUT | /api/rules/:id | 更新规则 |
| DELETE | /api/rules/:id | 删除规则 |
| POST | /api/rules/:id/start | 启动规则 |
| POST | /api/rules/:id/stop | 停止规则 |
| GET | /api/status | 获取状态 |
| PUT | /api/password | 修改密码 |
| GET | /api/config | 获取配置 |

## 配置文件

数据目录:
- Windows: `C:\Users\<User>\.transforward\`
- Linux/macOS: `~/.transforward/`

配置文件: `config.json`

```json
{
  "web_port": 8081,
  "password_hash": "$2a$10$...",
  "rules": [],
  "log_level": "info"
}
```

## 规则配置

```json
{
  "id": "1",
  "name": "Web Forward",
  "protocol": "tcp",
  "listen": ":8081",
  "target": "192.168.1.100:80",
  "enable": true
}
```

| 字段 | 说明 |
|------|------|
| id | 规则唯一标识 |
| name | 显示名称 |
| protocol | tcp / udp / tcp+udp |
| listen | 监听地址，格式 `:8081` |
| target | 目标地址，格式 `192.168.1.100:80` |
| enable | 是否自动启动 |

## 命令行参数

| 参数 | 说明 |
|------|------|
| -port=8080 | 指定自定义端口（会保存到配置） |
| -install | 安装服务 |
| -uninstall | 卸载服务 |
| -start | 启动服务 |
| -stop | 停止服务 |
| -restart | 重启服务 |
| -reset | 重置密码（交互式） |
| -version | 查看版本 |

## GitHub Actions

项目使用 GitHub Actions 进行自动构建:

- 推送 main/master 分支时自动构建
- 创建 v* 标签时自动发布 Release
- 支持多平台 (Ubuntu/Windows/macOS) 和多 Go 版本

## 架构

- **转发引擎**: 基于 goroutine 的高性能转发
- **WebSocket**: 实时状态推送 (3秒间隔)
- **认证**: bcrypt 密码哈希 + Bearer Token

## License

MIT
