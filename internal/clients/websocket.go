package clients

import (
	"context"
	"fmt"

	ws "debtster-export/internal/transport/websocket"
)

type WebSocketClient struct {
	hub *ws.Hub
}

func NewWebSocketClient(hub *ws.Hub) *WebSocketClient {
	return &WebSocketClient{
		hub: hub,
	}
}

func (c *WebSocketClient) NotifyExportProgress(
	ctx context.Context,
	userID int64,
	exportID string,
	progress float64,
	stage string,
) error {
	if c.hub == nil {
		return nil
	}

	channel := fmt.Sprintf("notify_user_of_progress_export#%d", userID)
	data := map[string]interface{}{
		"id":       exportID,
		"progress": progress,
	}
	if stage != "" {
		data["stage"] = stage
	}

	message := &ws.Message{
		Type:    "export_progress",
		Channel: channel,
		Data:    data,
	}

	c.hub.Broadcast(userID, message)
	return nil
}

func (c *WebSocketClient) NotifyExportComplete(
	ctx context.Context,
	userID int64,
	exportID string,
	url string,
	filename string,
) error {
	if c.hub == nil {
		return nil
	}

	channel := fmt.Sprintf("notify_user_when_export_complete#%d", userID)
	message := &ws.Message{
		Type:    "export_complete",
		Channel: channel,
		Data: map[string]interface{}{
			"id":       exportID,
			"url":      url,
			"filename": filename,
			"user_id":  userID,
		},
	}

	c.hub.Broadcast(userID, message)
	return nil
}

// NotifyExportFailed notifies a user that an export failed with the provided error message.
func (c *WebSocketClient) NotifyExportFailed(ctx context.Context, userID int64, exportID string, errMsg string) error {
	if c.hub == nil {
		return nil
	}

	channel := fmt.Sprintf("notify_user_when_export_failed#%d", userID)
	message := &ws.Message{
		Type:    "export_failed",
		Channel: channel,
		Data: map[string]interface{}{
			"id":      exportID,
			"message": errMsg,
			"user_id": userID,
		},
	}

	c.hub.Broadcast(userID, message)
	return nil
}
