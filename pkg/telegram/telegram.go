package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// MessageHandler is called when a message is received
type MessageHandler func(ctx context.Context, chatID int64, message string) (string, error)

// Gateway handles Telegram bot communication
type Gateway struct {
	bot     *tgbotapi.BotAPI
	handler MessageHandler
}

// New creates a new Telegram gateway
func New(token string, handler MessageHandler) (*Gateway, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)

	return &Gateway{
		bot:     bot,
		handler: handler,
	}, nil
}

// Start begins listening for messages
func (g *Gateway) Start(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := g.bot.GetUpdatesChan(u)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if update.Message == nil {
				continue
			}

			// Handle the message in a goroutine to avoid blocking
			go g.handleMessage(ctx, update.Message)
		}
	}
}

// handleMessage processes an incoming message
func (g *Gateway) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := msg.Text

	log.Printf("[%d] %s: %s", chatID, msg.From.UserName, text)

	// Call the handler
	response, err := g.handler(ctx, chatID, text)
	if err != nil {
		log.Printf("Error handling message: %v", err)
		response = fmt.Sprintf("Error: %v", err)
	}

	// Send the response
	if err := g.SendMessage(chatID, response); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

// SendMessage sends a text message to a chat
func (g *Gateway) SendMessage(chatID int64, text string) error {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	
	_, err := g.bot.Send(msg)
	return err
}

// GetChatID converts a string chat ID to int64
func GetChatID(chatIDStr string) (int64, error) {
	return strconv.ParseInt(chatIDStr, 10, 64)
}
