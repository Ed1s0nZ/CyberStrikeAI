package robot

// MessageHandler 供飞书/钉钉长连接调用的消息处理接口（由 handler.RobotHandler 实现）
type MessageHandler interface {
	HandleMessage(platform, userID, text string) string
}
