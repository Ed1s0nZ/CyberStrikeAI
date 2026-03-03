package robot

// MessageHandler is the message handling interface called by the Lark/DingTalk long connections (implemented by handler.RobotHandler)
type MessageHandler interface {
	HandleMessage(platform, userID, text string) string
}

// StreamingMessageHandler extends MessageHandler with streaming progress support.
// progressFn is called with short step descriptions during agent execution (e.g. "calling tool: nmap"),
// allowing platforms like Telegram to show live progress updates before the final reply arrives.
type StreamingMessageHandler interface {
	MessageHandler
	HandleMessageStream(platform, userID, text string, progressFn func(step string)) string
}
