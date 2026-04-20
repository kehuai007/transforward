# TransForward 测试报告

## 测试时间
2026-04-20

## 测试环境
- OS: Windows 10 Pro
- Go: 1.21+
- 测试目录: C:\Users\Admin\src\transforward

## 功能测试结果

### 1. Web UI 访问
- **状态**: ✓ PASS
- **说明**: 访问 http://localhost:8081/ 显示登录页面

### 2. 用户认证
- **状态**: ✓ PASS
- **测试方法**: POST /api/login
- **密码**: testpass123
- **响应**: {"token":"..."} - Token生成成功

### 3. 规则 CRUD
| 操作 | 状态 | 响应 |
|------|------|------|
| GET /api/rules | ✓ PASS | 返回规则数组 |
| POST /api/rules | ✓ PASS | {"success":true} |
| PUT /api/rules/:id | ✓ PASS | {"success":true} |
| DELETE /api/rules/:id | ✓ PASS | {"success":true} |

### 4. 状态查询
- **状态**: ✓ PASS
- **API**: GET /api/status
- **响应示例**:
```json
{
  "total_rules": 1,
  "active_rules": 1,
  "total_conns": 1,
  "total_bytes_in": 5,
  "total_bytes_out": 103,
  "rule_stats": [...]
}
```

### 5. TCP 流量转发
- **状态**: ✓ PASS
- **测试配置**: :19099 -> 127.0.0.1:8081
- **测试数据**: "HELLO" (5字节)
- **结果**: 5字节成功转发到目标，103字节响应返回

### 6. UDP 流量转发
- **状态**: ✓ PASS (修复后)
- **测试配置**: :19098 -> 127.0.0.1:8081
- **测试数据**: "UDPTEST" (7字节)
- **结果**: 数据成功发送，流量统计正确 (bytes_in=7, bytes_out=7)
- **说明**: UDP无连接，目标无响应时客户端超时属正常

### 7. TCP+UDP 流量转发
- **状态**: ✓ PASS
- **测试配置**: :19097 -> 127.0.0.1:8081
- **日志确认**:
```
[tcp+udp] UDP listening on :19097 -> 127.0.0.1:8081
[tcp+udp] TCP listening on :19097 -> 127.0.0.1:8081
```
- **结果**: TCP和UDP同时监听同一端口，转发功能正常

### 8. WebSocket
- **状态**: ✓ PASS
- **端点**: /ws
- **说明**: WebSocket升级响应正常

### 9. 配置持久化
- **状态**: ✓ PASS
- **存储位置**: .transforwardd/config.json (在运行目录下)
- **重启测试**: 重启后规则自动加载

### 10. 密码重置
- **状态**: ✓ PASS
- **命令**: transforward.exe -reset

## 测试用例脚本

### Windows 批处理测试
```
test\integration_test.bat
```

### Linux/macOS Shell 测试
```
test/integration_test.sh
```

## 手动测试命令

```bash
# 构建
go build -o transforward.exe .

# 设置密码
echo testpass123 | ./transforward.exe -reset

# 启动服务
./transforward.exe

# 登录 (获取token)
curl -X POST http://localhost:8081/api/login \
  -H "Content-Type: application/json" \
  -d '{"password":"testpass123"}'

# 添加TCP规则
curl -X POST http://localhost:8081/api/rules \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"id":"test-tcp","name":"Test TCP","protocol":"tcp","listen":"19099","target":"127.0.0.1:8081","enable":true}'

# 添加UDP规则
curl -X POST http://localhost:8081/api/rules \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"id":"test-udp","name":"Test UDP","protocol":"udp","listen":"19098","target":"127.0.0.1:8081","enable":true}'

# 添加TCP+UDP规则
curl -X POST http://localhost:8081/api/rules \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -d '{"id":"test-both","name":"Test Both","protocol":"tcp+udp","listen":"19097","target":"127.0.0.1:8081","enable":true}'

# 查看状态
curl -X GET http://localhost:8081/api/status \
  -H "Authorization: Bearer <TOKEN>"
```

## 已知问题

无

## 总结

| 测试项 | 结果 |
|--------|------|
| Web UI | ✓ 通过 |
| 用户认证 | ✓ 通过 |
| 规则管理 | ✓ 通过 |
| 状态查询 | ✓ 通过 |
| TCP转发 | ✓ 通过 |
| UDP转发 | ✓ 通过 |
| TCP+UDP转发 | ✓ 通过 |
| WebSocket | ✓ 通过 |
| 配置持久化 | ✓ 通过 |
| 密码重置 | ✓ 通过 |

**总计: 10/10 通过**

所有核心功能测试通过，TransForward 服务可以正常工作。
