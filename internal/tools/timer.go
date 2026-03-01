package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/vpoluyaktov/vibebot/internal/session"
	"github.com/yourorg/vibebot/pkg/telegram"
)

func init() {
	RegisterTool("set_timer", setTimerHandler)
}

// setTimerHandler implements a timer that triggers notifications after specified seconds
// Parameters:
// - duration: integer (required) - seconds to wait
// - message: string (optional) - notification content (default: "Timer expired!")
func setTimerHandler(ctx context.Context, args map[string]interface{}) (string, error) {
	duration, ok := args["duration"].(float64)
	if !ok {
		return "", fmt.Errorf("duration must be a number")
	}

	message, _ := args["message"].(string)
	if message == "" {
		message = "Timer expired!"
	}

	// Get session from context
	session := ctx.Value("session").(*session.Session)

	go func() {
		time.Sleep(time.Duration(duration) * time.Second)
		telegram.SendNotification(session.ChatID, message)
	}()

	return fmt.Sprintf("Timer set for %.0f seconds with message: %s", duration, message), nil
}