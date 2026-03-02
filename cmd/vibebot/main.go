package main

import (
	"github.com/vpoluyaktov/vibebot/internal/tasks"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/vpoluyaktov/vibebot/internal/agent"
	"github.com/vpoluyaktov/vibebot/internal/config"
	"github.com/vpoluyaktov/vibebot/internal/llm"
	"github.com/vpoluyaktov/vibebot/internal/logger"
	"github.com/vpoluyaktov/vibebot/internal/memory"
	"github.com/vpoluyaktov/vibebot/internal/modelmanager"
	"github.com/vpoluyaktov/vibebot/internal/session"
	"github.com/vpoluyaktov/vibebot/internal/telegram"
	"github.com/vpoluyaktov/vibebot/internal/tools"
)

func main() {
	fmt.Println("🤖 vibebot - AI assistant")

	if len(os.Args) < 2 {
		fmt.Println("Usage: vibebot <command>")
		fmt.Println("Commands:")
		fmt.Println("  gateway    Start the bot gateway")
		fmt.Println("  version    Show version info")
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "gateway":
		runGateway()
	case "version":
		showVersion()
	default:
		fmt.Printf("Unknown command: %s\n", command)
		os.Exit(1)
	}
}

func runGateway() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	if err := logger.SetLevelFromString(cfg.LogLevel); err != nil {
		fmt.Printf("Invalid log level '%s', using INFO\n", cfg.LogLevel)
		logger.SetLevel(logger.INFO)
	}
	logger.DisableStdLog() // Disable standard library log output

	logger.Info("Workspace: %s", cfg.WorkspaceDir)
	logger.Debug("Log level: %s", cfg.LogLevel)

	// Initialize memory
	mem, err := memory.New(cfg.WorkspaceDir)
	if err != nil {
		logger.Fatal("Failed to initialize memory: %v", err)
	}

	// Initialize session manager
	sessionMgr, err := session.NewManager(cfg.WorkspaceDir)
	if err != nil {
		logger.Fatal("Failed to initialize session manager: %v", err)
	}

	// Initialize model manager (will restore saved model if available)
	// Use first allowed model as default for initial startup
	defaultModel := cfg.OpenRouterAllowedModels[0]
	modelMgr := modelmanager.New(defaultModel, cfg.OpenRouterAllowedModels, cfg.WorkspaceDir)
	logger.Info("Model: %s", modelMgr.GetCurrent())

	// Initialize LLM provider with the current model from manager (may be restored from disk)
	provider := llm.NewOpenRouter(cfg.OpenRouterAPIKey, modelMgr.GetCurrent())
	logger.Info("Current model: %s", modelMgr.GetCurrent())

	// Fetch context lengths for all allowed models
	ctx := context.Background()
	contextLengths, err := provider.FetchModelContextLengths(ctx, cfg.OpenRouterAllowedModels)
	if err != nil {
		logger.Warn("Failed to fetch model context lengths: %v (stats will not show context percentage)", err)
	} else {
		modelMgr.SetContextLengths(contextLengths)
		logger.Debug("Fetched context lengths for %d models", len(contextLengths))
	}

	// Initialize task manager
	taskMgr := tasks.NewManager(cfg.WorkspaceDir + "/tasks")

	// Initialize Telegram gateway FIRST (needed for timer tool)
	// We'll set the actual handler after agent is created
	tg, err := telegram.New(cfg.TelegramToken, nil, cfg.TelegramAllowedUsers)
	if err != nil {
		logger.Fatal("Failed to initialize Telegram gateway: %v", err)
	}

	// Initialize tool registry
	toolRegistry := tools.NewRegistry()
	tools.RegisterFileTools(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterExecTool(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterBatchTools(toolRegistry)

	// Register optimization tools
	tools.RegisterMultiFileRead(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterSearchAndRead(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterCodeContext(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterDiffPreview(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterWorkspaceSnapshot(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterSmartEdit(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterTestAndFix(toolRegistry, cfg.WorkspaceDir)

	// Register token-saving tools
	tools.RegisterFileSummary(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterFileOutline(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterSymbolDefinition(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterCachedGrep(toolRegistry, cfg.WorkspaceDir)
	tools.RegisterIncrementalEdit(toolRegistry, cfg.WorkspaceDir)

	// Register timer tool BEFORE agent creation (needs telegram gateway and task manager)
	tools.RegisterTimerTool(toolRegistry, tg, taskMgr)

	// Initialize agent (with all tools registered)
	ag := agent.New(provider, mem, toolRegistry, sessionMgr, modelMgr, cfg.WorkspaceDir, taskMgr)

	// Set up task resumption handler
	taskMgr.SetResumeHandler(func(ctx context.Context, chatID int64, taskContext string) error {
		return ag.ResumeTask(ctx, chatID, taskContext)
	})

	// Create message handler
	handler := func(ctx context.Context, chatID int64, message string) (string, error) {
		return ag.ProcessMessage(ctx, chatID, message)
	}

	// Now set the handler on the telegram gateway
	tg.SetHandler(handler)

	// Set up progress callback to send intermediate updates
	ag.SetProgressCallback(func(chatID int64, message string, isToolHint bool) {
		if err := tg.SendProgressMessage(chatID, message, isToolHint); err != nil {
			logger.Debug("Failed to send progress message: %v", err)
		}
	})

	// Set up keyboard sender for inline keyboards (using adapter to avoid circular dependency)
	keyboardAdapter := &telegramKeyboardAdapter{gateway: tg}
	ag.SetKeyboardSender(keyboardAdapter)

	// Set up callback handler for inline keyboard button presses
	callbackHandler := func(ctx context.Context, chatID int64, callbackData string) (string, error) {
		return ag.ProcessCallback(ctx, chatID, callbackData)
	}
	tg.SetCallbackHandler(callbackHandler)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutting down...")
		cancel()
	}()

	// Send restart notifications to all users
	logger.Info("Sending restart notifications...")
	if chatIDs, err := sessionMgr.GetAllChatIDs(); err == nil {
		for _, chatID := range chatIDs {
			if err := tg.SendMessage(chatID, "🤖 vibebot service restarted and ready to use"); err != nil {
				logger.Warn("Failed to send restart notification to chat %d: %v", chatID, err)
			} else {
				logger.Debug("Sent restart notification to chat %d", chatID)
			}
		}
	} else {
		logger.Warn("Failed to get chat IDs for restart notifications: %v", err)
	}

	// Start the gateway
	logger.Info("Starting Telegram gateway...")
	if err := tg.Start(ctx); err != nil && err != context.Canceled {
		logger.Fatal("Gateway error: %v", err)
	}

	logger.Info("Shutdown complete")
}

// telegramKeyboardAdapter adapts telegram.Gateway to agent.KeyboardSender interface
type telegramKeyboardAdapter struct {
	gateway *telegram.Gateway
}

func (a *telegramKeyboardAdapter) SendMessageWithKeyboard(chatID int64, text string, keyboard [][]telegram.InlineButton) error {
	return a.gateway.SendMessageWithKeyboard(chatID, text, keyboard)
}

func showVersion() {
	fmt.Println("vibebot v0.1.0")
	fmt.Println("A modern AI assistant written in Go")
}
