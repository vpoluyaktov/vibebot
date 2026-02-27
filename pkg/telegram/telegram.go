package telegram

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vpoluyaktov/vibebot/internal/logger"
)

// MessageHandler is called when a message is received
type MessageHandler func(ctx context.Context, chatID int64, message string) (string, error)

// Gateway handles Telegram bot communication
type Gateway struct {
	bot             *tgbotapi.BotAPI
	handler         MessageHandler
	allowedUsers    []int64
	processedMsgIDs map[int]bool // Track processed message IDs to prevent duplicates
	msgMutex        sync.Mutex   // Protects processedMsgIDs map
}

// New creates a new Telegram gateway
func New(token string, handler MessageHandler, allowedUsers []int64) (*Gateway, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	logger.Info("Authorized on account %s", bot.Self.UserName)
	
	if len(allowedUsers) > 0 {
		logger.Info("User whitelist enabled: %v", allowedUsers)
	} else {
		logger.Warn("No user whitelist configured - accepting messages from all users")
	}

	// Register bot commands for the command menu
	commands := []tgbotapi.BotCommand{
		{Command: "start", Description: "Start the bot"},
		{Command: "new", Description: "Start a new conversation"},
		{Command: "model", Description: "Show/switch LLM model"},
		{Command: "models", Description: "List available models"},
		{Command: "help", Description: "Show available commands"},
		{Command: "projects", Description: "List existing projects"},
		{Command: "project", Description: "Switch between projects or create/delete projects"},
	}
	
	cfg := tgbotapi.NewSetMyCommands(commands...)
	if _, err := bot.Request(cfg); err != nil {
		logger.Warn("Failed to register bot commands: %v", err)
	} else {
		logger.Debug("Bot commands registered successfully")
	}

	return &Gateway{
		bot:             bot,
		handler:         handler,
		allowedUsers:    allowedUsers,
		processedMsgIDs: make(map[int]bool),
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

			// Check and mark message as processed atomically to prevent duplicates
			g.msgMutex.Lock()
			msgID := update.Message.MessageID
			if g.processedMsgIDs[msgID] {
				g.msgMutex.Unlock()
				logger.Debug("Skipping duplicate message ID: %d", msgID)
				continue
			}
			logger.Debug("Marking message ID %d as processed", msgID)
			g.processedMsgIDs[msgID] = true
			g.msgMutex.Unlock()

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

	logger.Info("[%d] %s (ID: %d): %s", chatID, msg.From.UserName, userID, text)

	// Check if user is allowed
	if !g.isUserAllowed(userID) {
		logger.Warn("Unauthorized access attempt from user %d (%s)", userID, msg.From.UserName)
		response := "⛔ Unauthorized. This bot is private.\n\nYour Telegram ID: " + strconv.FormatInt(userID, 10)
		if err := g.SendMessage(chatID, response); err != nil {
			logger.Error("Error sending unauthorized message: %v", err)
		}
		return
	}

	logger.Debug("Processing message from chat %d: %s", chatID, text)

	// Start typing indicator
	stopTyping := g.startTypingIndicator(ctx, chatID)
	defer stopTyping()

	// Call the handler
	response, err := g.handler(ctx, chatID, text)
	if err != nil {
		logger.Error("Error handling message: %v", err)
		response = fmt.Sprintf("Error: %v", err)
	}

	logger.Debug("Handler returned response (length: %d): %s", len(response), response)

	// Send the response
	if err := g.SendMessage(chatID, response); err != nil {
		logger.Error("Error sending message: %v", err)
	} else {
		logger.Debug("Response sent successfully to chat %d", chatID)
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

// sendChatAction sends a chat action (like typing) to a chat
func (g *Gateway) sendChatAction(chatID int64, action string) error {
	chatAction := tgbotapi.NewChatAction(chatID, action)
	_, err := g.bot.Request(chatAction)
	return err
}

// startTypingIndicator starts sending typing indicator and returns a stop function
func (g *Gateway) startTypingIndicator(ctx context.Context, chatID int64) func() {
	// Create a context for the typing indicator
	typingCtx, cancel := context.WithCancel(ctx)
	
	// Start goroutine to send typing indicator every 5 seconds
	go func() {
		// Send initial typing indicator
		if err := g.sendChatAction(chatID, "typing"); err != nil {
			logger.Debug("Failed to send typing indicator: %v", err)
		}
		
		// Continue sending every 5 seconds until stopped
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				if err := g.sendChatAction(chatID, "typing"); err != nil {
					logger.Debug("Failed to send typing indicator: %v", err)
				}
			}
		}
	}()
	
	// Return the stop function
	return cancel
}

// markdownToTelegramHTML converts markdown to Telegram-safe HTML
func markdownToTelegramHTML(text string) string {
	if text == "" {
		return ""
	}

	// 1. Extract and protect code blocks
	var codeBlocks []string
	codeBlockRe := regexp.MustCompile("```[\\w]*\\n?([\\s\\S]*?)```")
	text = codeBlockRe.ReplaceAllStringFunc(text, func(m string) string {
		matches := codeBlockRe.FindStringSubmatch(m)
		if len(matches) > 1 {
			codeBlocks = append(codeBlocks, matches[1])
			return fmt.Sprintf("\x00CB%d\x00", len(codeBlocks)-1)
		}
		return m
	})

	// 2. Extract and protect inline code
	var inlineCodes []string
	inlineCodeRe := regexp.MustCompile("`([^`]+)`")
	text = inlineCodeRe.ReplaceAllStringFunc(text, func(m string) string {
		matches := inlineCodeRe.FindStringSubmatch(m)
		if len(matches) > 1 {
			inlineCodes = append(inlineCodes, matches[1])
			return fmt.Sprintf("\x00IC%d\x00", len(inlineCodes)-1)
		}
		return m
	})

	// 3. Headers # Title -> just the title text
	headerRe := regexp.MustCompile("(?m)^#{1,6}\\s+(.+)$")
	text = headerRe.ReplaceAllString(text, "$1")

	// 4. Blockquotes > text -> just the text
	blockquoteRe := regexp.MustCompile("(?m)^>\\s*(.*)$")
	text = blockquoteRe.ReplaceAllString(text, "$1")

	// 5. Escape HTML special characters
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "<", "&lt;")
	text = strings.ReplaceAll(text, ">", "&gt;")

	// 6. Links [text](url)
	linkRe := regexp.MustCompile("\\[([^\\]]+)\\]\\(([^)]+)\\)")
	text = linkRe.ReplaceAllString(text, `<a href="$2">$1</a>`)

	// 7. Bold **text** or __text__
	boldRe1 := regexp.MustCompile("\\*\\*(.+?)\\*\\*")
	text = boldRe1.ReplaceAllString(text, "<b>$1</b>")
	boldRe2 := regexp.MustCompile("__(.+?)__")
	text = boldRe2.ReplaceAllString(text, "<b>$1</b>")

	// 8. Italic *text* (single asterisk, not already bold)
	italicRe := regexp.MustCompile("(?:^|\\s)\\*([^*]+)\\*(?:$|\\s)")
	text = italicRe.ReplaceAllString(text, " <i>$1</i> ")

	// 9. Strikethrough ~~text~~
	strikeRe := regexp.MustCompile("~~(.+?)~~")
	text = strikeRe.ReplaceAllString(text, "<s>$1</s>")

	// 10. Bullet lists - item -> • item
	bulletRe := regexp.MustCompile("(?m)^[-*]\\s+")
	text = bulletRe.ReplaceAllString(text, "• ")

	// 11. Restore inline code with HTML tags
	for i, code := range inlineCodes {
		escaped := strings.ReplaceAll(code, "&", "&amp;")
		escaped = strings.ReplaceAll(escaped, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		text = strings.ReplaceAll(text, fmt.Sprintf("\x00IC%d\x00", i), fmt.Sprintf("<code>%s</code>", escaped))
	}

	// 12. Restore code blocks with HTML tags
	for i, code := range codeBlocks {
		escaped := strings.ReplaceAll(code, "&", "&amp;")
		escaped = strings.ReplaceAll(escaped, "<", "&lt;")
		escaped = strings.ReplaceAll(escaped, ">", "&gt;")
		text = strings.ReplaceAll(text, fmt.Sprintf("\x00CB%d\x00", i), fmt.Sprintf("<pre><code>%s</code></pre>", escaped))
	}

	return text
}

// SendMessage sends a text message to a chat
func (g *Gateway) SendMessage(chatID int64, text string) error {
	// Convert markdown to Telegram HTML
	html := markdownToTelegramHTML(text)
	
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = "HTML"
	
	_, err := g.bot.Send(msg)
	if err != nil {
		// If HTML parsing fails, fall back to plain text
		logger.Warn("HTML parse failed, falling back to plain text: %v", err)
		msg.Text = text
		msg.ParseMode = ""
		_, err = g.bot.Send(msg)
	}
	
	return err
}

// GetChatID converts a string chat ID to int64
func GetChatID(chatIDStr string) (int64, error) {
	return strconv.ParseInt(chatIDStr, 10, 64)
}
