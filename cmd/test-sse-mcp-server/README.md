# SSE MCP 测试服务器

这是一个用于验证SSE模式外部MCP功能的测试服务器。

## 使用方法

### 1. 启动测试服务器

```bash
cd cmd/test-sse-mcp-server
go run main.go
```

服务器将在 `http://127.0.0.1:8082` 启动，提供以下端点：
- `GET /sse` - SSE事件流端点
- `POST /message` - 消息接收端点

### 2. 在能盾智御中添加配置

在Web界面中添加外部MCP配置，使用以下JSON：

```json
{
  "test-sse-mcp": {
    "transport": "sse",
    "url": "http://127.0.0.1:8082/sse",
    "description": "SSE MCP测试服务器",
    "timeout": 30
  }
}
```

### 3. 测试功能

测试服务器提供两个测试工具：

1. **test_echo** - 回显输入的文本
   - 参数：`text` (string) - 要回显的文本

2. **test_add** - 计算两个数字的和
   - 参数：`a` (number) - 第一个数字
   - 参数：`b` (number) - 第二个数字

## 工作原理

1. 客户端通过 `GET /sse` 建立SSE连接，接收服务器推送的事件
2. 客户端通过 `POST /message` 发送MCP协议消息
3. 服务器处理消息后，通过SSE连接推送响应

## 日志

服务器会输出以下日志：
- SSE客户端连接/断开
- 收到的请求（方法名和ID）
- 工具调用详情
