package clients

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	ws "debtster-export/internal/transport/websocket"

	"github.com/gorilla/websocket"
)

func TestWebSocketClient_NotifyExportProgress(t *testing.T) {
	hub := ws.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	// Создаем тестовый websocket сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r, 1)
	}))
	defer server.Close()

	// Подключаемся к websocket
	wsURL := "ws" + server.URL[4:] + "?user_id=1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Даем время на регистрацию
	time.Sleep(100 * time.Millisecond)

	// Создаем клиент
	client := NewWebSocketClient(hub)

	// Отправляем уведомление о прогрессе
	err = client.NotifyExportProgress(context.Background(), 1, "export-123", 50.5, "")
	if err != nil {
		t.Fatalf("Failed to notify progress: %v", err)
	}

	// Читаем сообщение
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	var received ws.Message
	err = conn.ReadJSON(&received)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	// Проверяем структуру сообщения
	if received.Type != "export_progress" {
		t.Errorf("Expected type 'export_progress', got '%s'", received.Type)
	}
	if received.UserID != 1 {
		t.Errorf("Expected userID 1, got %d", received.UserID)
	}
	if received.Channel != "notify_user_of_progress_export#1" {
		t.Errorf("Expected channel 'notify_user_of_progress_export#1', got '%s'", received.Channel)
	}

	// Проверяем данные
	dataBytes, err := json.Marshal(received.Data)
	if err != nil {
		t.Fatalf("Failed to marshal data: %v", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(dataBytes, &data)
	if err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if data["id"] != "export-123" {
		t.Errorf("Expected id 'export-123', got '%v'", data["id"])
	}
	if data["progress"].(float64) != 50.5 {
		t.Errorf("Expected progress 50.5, got %v", data["progress"])
	}
}

func TestWebSocketClient_NotifyExportComplete(t *testing.T) {
	hub := ws.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	// Создаем тестовый websocket сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r, 1)
	}))
	defer server.Close()

	// Подключаемся к websocket
	wsURL := "ws" + server.URL[4:] + "?user_id=1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Даем время на регистрацию
	time.Sleep(100 * time.Millisecond)

	// Создаем клиент
	client := NewWebSocketClient(hub)

	// Отправляем уведомление о завершении
	err = client.NotifyExportComplete(context.Background(), 1, "export-123", "https://example.com/file.xlsx", "debts_20240101.xlsx")
	if err != nil {
		t.Fatalf("Failed to notify complete: %v", err)
	}

	// Читаем сообщение
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	var received ws.Message
	err = conn.ReadJSON(&received)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	// Проверяем структуру сообщения
	if received.Type != "export_complete" {
		t.Errorf("Expected type 'export_complete', got '%s'", received.Type)
	}
	if received.UserID != 1 {
		t.Errorf("Expected userID 1, got %d", received.UserID)
	}
	if received.Channel != "notify_user_when_export_complete#1" {
		t.Errorf("Expected channel 'notify_user_when_export_complete#1', got '%s'", received.Channel)
	}

	// Проверяем данные
	dataBytes, err := json.Marshal(received.Data)
	if err != nil {
		t.Fatalf("Failed to marshal data: %v", err)
	}

	var data map[string]interface{}
	err = json.Unmarshal(dataBytes, &data)
	if err != nil {
		t.Fatalf("Failed to unmarshal data: %v", err)
	}

	if data["id"] != "export-123" {
		t.Errorf("Expected id 'export-123', got '%v'", data["id"])
	}
	if data["url"] != "https://example.com/file.xlsx" {
		t.Errorf("Expected url 'https://example.com/file.xlsx', got '%v'", data["url"])
	}
	if data["filename"] != "debts_20240101.xlsx" {
		t.Errorf("Expected filename 'debts_20240101.xlsx', got '%v'", data["filename"])
	}
	if int64(data["user_id"].(float64)) != 1 {
		t.Errorf("Expected user_id 1, got %v", data["user_id"])
	}
}

func TestWebSocketClient_NilHub(t *testing.T) {
	// Создаем клиент с nil hub
	client := NewWebSocketClient(nil)

	// Должно работать без ошибок
	err := client.NotifyExportProgress(context.Background(), 1, "export-123", 50.5, "")
	if err != nil {
		t.Errorf("Should not return error with nil hub, got: %v", err)
	}

	err = client.NotifyExportComplete(context.Background(), 1, "export-123", "https://example.com/file.xlsx", "file.xlsx")
	if err != nil {
		t.Errorf("Should not return error with nil hub, got: %v", err)
	}
}

func TestWebSocketClient_NotifyExportFailed(t *testing.T) {
	hub := ws.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r, 1)
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:] + "?user_id=1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Give time for registration
	time.Sleep(50 * time.Millisecond)

	client := NewWebSocketClient(hub)

	err = client.NotifyExportFailed(context.Background(), 1, "export-123", "upload failed")
	if err != nil {
		t.Fatalf("Failed to notify failed: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	var received ws.Message
	err = conn.ReadJSON(&received)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	if received.Type != "export_failed" {
		t.Errorf("Expected type 'export_failed', got '%s'", received.Type)
	}
	if received.Channel != "notify_user_when_export_failed#1" {
		t.Errorf("Expected channel 'notify_user_when_export_failed#1', got '%s'", received.Channel)
	}

	dataBytes, _ := json.Marshal(received.Data)
	var data map[string]interface{}
	_ = json.Unmarshal(dataBytes, &data)

	if data["id"] != "export-123" {
		t.Errorf("Expected id 'export-123', got '%v'", data["id"])
	}
	if data["message"] != "upload failed" {
		t.Errorf("Expected message 'upload failed', got '%v'", data["message"])
	}
}

func TestWebSocketClient_MultipleProgressUpdates(t *testing.T) {
	hub := ws.NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	// Создаем тестовый websocket сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.HandleWebSocket(w, r, 1)
	}))
	defer server.Close()

	// Подключаемся к websocket
	wsURL := "ws" + server.URL[4:] + "?user_id=1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Даем время на регистрацию
	time.Sleep(100 * time.Millisecond)

	// Создаем клиент
	client := NewWebSocketClient(hub)

	// Отправляем несколько обновлений прогресса
	progresses := []float64{10.0, 25.0, 50.0, 75.0, 100.0}
	for _, progress := range progresses {
		err = client.NotifyExportProgress(context.Background(), 1, "export-123", progress, "")
		if err != nil {
			t.Fatalf("Failed to notify progress: %v", err)
		}

		// Читаем сообщение
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		var received ws.Message
		err = conn.ReadJSON(&received)
		if err != nil {
			t.Fatalf("Failed to read message: %v", err)
		}

		// Проверяем прогресс
		dataBytes, _ := json.Marshal(received.Data)
		var data map[string]interface{}
		json.Unmarshal(dataBytes, &data)

		if data["progress"].(float64) != progress {
			t.Errorf("Expected progress %.1f, got %.1f", progress, data["progress"].(float64))
		}
	}
}
