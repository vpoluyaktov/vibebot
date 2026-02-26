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
	bot          *tgbotapi.BotAPI
	handler      MessageHandler
	allowedUsers []int64
}

// New creates a new Telegram gateway
func New(token string, handler MessageHandler, allowedUsers []int64) (*Gateway, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	log.Printf("Authorized on account %s", bot.Self.UserName)
	
	if len(allowedUsers) > 0 {
		log.Printf("User whitelist enabled: %v", allowedUsers)
	} else {
		log.Printf("Warning: No user whitelist configured - accepting messages from all users")
	}

	return &Gateway{
		bot:          bot,
		handler:      handler,
		allowedUsers: allowedUsers,
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
	userID := msg.From.ID
	text := msg.Text

	log.Printf("[%d] %s (ID: %d): %s", chatID, msg.From.UserName, userID, text)

	// Check if user is allowed
	if !g.isUserAllowed(userID) {
		log.Printf("Unauthorized access attempt from user %d (%s)", userID, msg.From.UserName)
		response := "⛔ Unauthorized. This bot is private.\n\nYour Telegram ID: " + strconv.FormatInt(userID, 10)
		if err := g.SendMessage(chatID, response); err != nil {
			log.Printf("Error sending unauthorized message: %v", err)
		}
		return
	}

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

// isUserAllowed checks if a user is in the whitelist
func (g *Gateway) isUserAllowed(userID int64) bool {
	// If no whitelist configured, allow all users
	if len(g.allowedUsers) == 0 {
		return true
	}
	
	// Check if user is in the whitelist
	for _, allowedID := range g.allowedUsers {
		if allowedID == userID {
			return true
		}
	}
	
	return false
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
