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
	processedMsgIDs map[int]bool            // Track processed message IDs to prevent duplicates
	msgMutex        sync.Mutex              // Protects processedMsgIDs map
	chatLocks       map[int64]*sync.Mutex   // Per-chat locks to prevent parallel processing
	chatLocksMutex  sync.Mutex              // Protects chatLocks map
	processingState map[int64]bool          // Track if a chat is currently processing
	stateMutex      sync.Mutex              // Protects processingState map
	stopSignals     map[int64]chan struct{} // Stop signals per chat
	stopMutex       sync.Mutex              // Protects stopSignals map
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
		{Command: "stop", Description: "Stop processing and clear queue"},
		{Command: "model", Description: "Show/switch LLM model"},
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
		chatLocks:       make(map[int64]*sync.Mutex),
		processingState: make(map[int64]bool),
		stopSignals:     make(map[int64]chan struct{}),
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

// getChatLock gets or creates a mutex for a specific chat
func (g *Gateway) getChatLock(chatID int64) *sync.Mutex {
	g.chatLocksMutex.Lock()
	defer g.chatLocksMutex.Unlock()

	if lock, exists := g.chatLocks[chatID]; exists {
		return lock
	}

	lock := &sync.Mutex{}
	g.chatLocks[chatID] = lock
	return lock
}

// handleMessage processes an incoming message
func (g *Gateway) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	userID := msg.From.ID
	text := msg.Text

	// Check if another message is being processed for this chat
	g.stateMutex.Lock()
	isProcessing := g.processingState[chatID]
	g.stateMutex.Unlock()

	// If processing, send immediate queue notification
	if isProcessing {
		queueMsg := "⏳ Previous message is still processing. Your message is queued and will be handled next."
		if err := g.SendMessage(chatID, queueMsg); err != nil {
			logger.Error("Error sending queue notification: %v", err)
		}
	}

	// Acquire per-chat lock to prevent parallel processing of messages from the same chat
	chatLock := g.getChatLock(chatID)
	chatLock.Lock()
	defer chatLock.Unlock()

	// Mark as processing
	g.stateMutex.Lock()
	g.processingState[chatID] = true
	g.stateMutex.Unlock()

	defer func() {
		// Clear processing state when done
		g.stateMutex.Lock()
		g.processingState[chatID] = false
		g.stateMutex.Unlock()
	}()

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

	// Handle /stop command
	if text == "/stop" {
		g.handleStopCommand(chatID)
		return
	}

	// Create stop signal channel for this chat
	g.stopMutex.Lock()
	stopChan := make(chan struct{})
	g.stopSignals[chatID] = stopChan
	g.stopMutex.Unlock()

	defer func() {
		// Clean up stop signal
		g.stopMutex.Lock()
		delete(g.stopSignals, chatID)
		g.stopMutex.Unlock()
	}()

	// Start typing indicator
	stopTyping := g.startTypingIndicator(ctx, chatID)
	defer stopTyping()

	// Call the handler
	response, err := g.handler(ctx, chatID, text)

	// Check if processing was stopped
	select {
	case <-stopChan:
		logger.Info("Message processing stopped for chat %d", chatID)
		return
	default:
		// Continue with response
	}
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

// handleStopCommand handles the /stop command to cancel processing
func (g *Gateway) handleStopCommand(chatID int64) {
	// Signal any ongoing processing to stop
	g.stopMutex.Lock()
	if stopChan, exists := g.stopSignals[chatID]; exists {
		close(stopChan)
		delete(g.stopSignals, chatID)
	}
	g.stopMutex.Unlock()

	logger.Info("Stop command received for chat %d", chatID)

	// Send confirmation
	response := "⛔ Processing stopped. Queue cleared."
	if err := g.SendMessage(chatID, response); err != nil {
		logger.Error("Error sending stop confirmation: %v", err)
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

// SendMessage sends a text message to a chat, splitting if necessary
func (g *Gateway) SendMessage(chatID int64, text string) error {
	// Telegram's message limit is 4096 characters
	const maxLength = 4000 // Leave some margin for HTML tags

	// Convert markdown to Telegram HTML
	html := markdownToTelegramHTML(text)

	// If message is short enough, send directly
	if len(html) <= maxLength {
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

	// Message is too long - split it
	logger.Info("Message too long (%d chars), splitting into chunks", len(html))
	chunks := splitMessage(text, maxLength)

	for i, chunk := range chunks {
		chunkHTML := markdownToTelegramHTML(chunk)
		msg := tgbotapi.NewMessage(chatID, chunkHTML)
		msg.ParseMode = "HTML"

		_, err := g.bot.Send(msg)
		if err != nil {
			// Fall back to plain text for this chunk
			logger.Warn("HTML parse failed for chunk %d, falling back to plain text: %v", i+1, err)
			msg.Text = chunk
			msg.ParseMode = ""
			_, err = g.bot.Send(msg)
			if err != nil {
				return fmt.Errorf("failed to send chunk %d: %w", i+1, err)
			}
		}

		// Small delay between chunks to avoid rate limiting
		if i < len(chunks)-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return nil
}

// SendProgressMessage sends a progress update message (lighter formatting for tool hints)
func (g *Gateway) SendProgressMessage(chatID int64, text string, isToolHint bool) error {
	// For tool hints, use monospace formatting
	if isToolHint {
		msg := tgbotapi.NewMessage(chatID, text)
		msg.ParseMode = ""
		_, err := g.bot.Send(msg)
		return err
	}

	// For regular progress, use light HTML formatting
	html := markdownToTelegramHTML(text)
	msg := tgbotapi.NewMessage(chatID, html)
	msg.ParseMode = "HTML"

	_, err := g.bot.Send(msg)
	if err != nil {
		// Fall back to plain text
		msg.Text = text
		msg.ParseMode = ""
		_, err = g.bot.Send(msg)
	}

	return err
}

// splitMessage splits a long message into chunks at natural boundaries
// Based on nanobot's simpler approach: prefer newlines, then spaces, then hard split
func splitMessage(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		if len(text) <= maxLength {
			chunks = append(chunks, text)
			break
		}

		// Try to split at a newline
		cut := text[:maxLength]
		pos := strings.LastIndex(cut, "\n")
		if pos == -1 {
			// No newline, try to split at a space
			pos = strings.LastIndex(cut, " ")
		}
		if pos == -1 {
			// No space either, hard split
			pos = maxLength
		}

		chunks = append(chunks, text[:pos])
		text = strings.TrimLeft(text[pos:], " \n")
	}

	return chunks
}

// GetChatID converts a string chat ID to int64
func GetChatID(chatIDStr string) (int64, error) {
	return strconv.ParseInt(chatIDStr, 10, 64)
}
