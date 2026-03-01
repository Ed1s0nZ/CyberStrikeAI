package robot

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"cyberstrike-ai/internal/config"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"go.uber.org/zap"
)

const (
	larkReconnectInitial = 5 * time.Second  // 首次重连间隔
	larkReconnectMax     = 60 * time.Second // 最大重连间隔
)

type larkTextContent struct {
	Text string `json:"text"`
}

// StartLark 启动飞书长连接（无需公网），收到消息后调用 handler 并回复。
// 断线（如笔记本睡眠、网络中断）后会自动重连；ctx 被取消时退出，便于配置变更时重启。
func StartLark(ctx context.Context, cfg config.RobotLarkConfig, h MessageHandler, logger *zap.Logger) {
	if !cfg.Enabled || cfg.AppID == "" || cfg.AppSecret == "" {
		return
	}
	go runLarkLoop(ctx, cfg, h, logger)
}

// runLarkLoop 循环维持飞书长连接：断开且 ctx 未取消时按退避间隔重连。
func runLarkLoop(ctx context.Context, cfg config.RobotLarkConfig, h MessageHandler, logger *zap.Logger) {
	backoff := larkReconnectInitial
	for {
		larkClient := lark.NewClient(cfg.AppID, cfg.AppSecret)
		eventHandler := dispatcher.NewEventDispatcher("", "").OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			go handleLarkMessage(ctx, event, h, larkClient, logger)
			return nil
		})
		wsClient := larkws.NewClient(cfg.AppID, cfg.AppSecret,
			larkws.WithEventHandler(eventHandler),
			larkws.WithLogLevel(larkcore.LogLevelInfo),
		)
		logger.Info("飞书长连接正在连接…", zap.String("app_id", cfg.AppID))
		err := wsClient.Start(ctx)
		if ctx.Err() != nil {
			logger.Info("飞书长连接已按配置重启关闭")
			return
		}
		if err != nil {
			logger.Warn("飞书长连接断开（如睡眠/断网），将自动重连", zap.Error(err), zap.Duration("retry_after", backoff))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
			if backoff < larkReconnectMax {
				backoff *= 2
				if backoff > larkReconnectMax {
					backoff = larkReconnectMax
				}
			}
		}
	}
}

func handleLarkMessage(ctx context.Context, event *larkim.P2MessageReceiveV1, h MessageHandler, client *lark.Client, logger *zap.Logger) {
	if event == nil || event.Event == nil || event.Event.Message == nil || event.Event.Sender == nil || event.Event.Sender.SenderId == nil {
		return
	}
	msg := event.Event.Message
	msgType := larkcore.StringValue(msg.MessageType)
	if msgType != larkim.MsgTypeText {
		logger.Debug("飞书暂仅处理文本消息", zap.String("msg_type", msgType))
		return
	}
	var textBody larkTextContent
	if err := json.Unmarshal([]byte(larkcore.StringValue(msg.Content)), &textBody); err != nil {
		logger.Warn("飞书消息 Content 解析失败", zap.Error(err))
		return
	}
	text := strings.TrimSpace(textBody.Text)
	if text == "" {
		return
	}
	userID := ""
	if event.Event.Sender.SenderId.UserId != nil {
		userID = *event.Event.Sender.SenderId.UserId
	}
	messageID := larkcore.StringValue(msg.MessageId)
	reply := h.HandleMessage("lark", userID, text)
	contentBytes, _ := json.Marshal(larkTextContent{Text: reply})
	_, err := client.Im.Message.Reply(ctx, larkim.NewReplyMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			Content(string(contentBytes)).
			Build()).
		Build())
	if err != nil {
		logger.Warn("飞书回复失败", zap.String("message_id", messageID), zap.Error(err))
		return
	}
	logger.Debug("飞书已回复", zap.String("message_id", messageID))
}
