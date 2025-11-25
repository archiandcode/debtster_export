package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHub_RegisterAndUnregister(t *testing.T) {
	hub := NewHub()
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

	// Проверяем, что подключение зарегистрировано
	hub.mu.RLock()
	connections, exists := hub.connections[1]
	hub.mu.RUnlock()

	if !exists {
		t.Fatal("Connection should be registered")
	}
	if len(connections) != 1 {
		t.Fatalf("Expected 1 connection, got %d", len(connections))
	}

	// Закрываем соединение
	conn.Close()

	// Даем время на отмену регистрации
	time.Sleep(100 * time.Millisecond)

	// Проверяем, что подключение удалено
	hub.mu.RLock()
	_, exists = hub.connections[1]
	hub.mu.RUnlock()

	if exists {
		t.Fatal("Connection should be unregistered")
	}
}

func TestHub_Broadcast(t *testing.T) {
	hub := NewHub()
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

	// Отправляем сообщение
	message := &Message{
		Type:    "test",
		Channel: "test_channel",
		Data:    map[string]interface{}{"test": "data"},
	}
	hub.Broadcast(1, message)

	// Читаем сообщение
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	var received Message
	err = conn.ReadJSON(&received)
	if err != nil {
		t.Fatalf("Failed to read message: %v", err)
	}

	if received.Type != "test" {
		t.Errorf("Expected type 'test', got '%s'", received.Type)
	}
	if received.Channel != "test_channel" {
		t.Errorf("Expected channel 'test_channel', got '%s'", received.Channel)
	}
	if received.UserID != 1 {
		t.Errorf("Expected userID 1, got %d", received.UserID)
	}
}

func TestHub_MultipleConnections(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	// Создаем тестовый websocket сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := int64(1)
		hub.HandleWebSocket(w, r, userID)
	}))
	defer server.Close()

	// Создаем несколько подключений
	var conns []*websocket.Conn
	for i := 0; i < 3; i++ {
		wsURL := "ws" + server.URL[4:] + "?user_id=1"
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}
		conns = append(conns, conn)
		defer conn.Close()
	}

	// Даем время на регистрацию
	time.Sleep(100 * time.Millisecond)

	// Проверяем, что все подключения зарегистрированы
	hub.mu.RLock()
	connections, exists := hub.connections[1]
	hub.mu.RUnlock()

	if !exists {
		t.Fatal("Connections should be registered")
	}
	if len(connections) != 3 {
		t.Fatalf("Expected 3 connections, got %d", len(connections))
	}

	// Отправляем сообщение
	message := &Message{
		Type:    "broadcast",
		Channel: "test",
		Data:    map[string]interface{}{"test": "data"},
	}
	hub.Broadcast(1, message)

	// Проверяем, что все подключения получили сообщение
	var wg sync.WaitGroup
	for i, conn := range conns {
		wg.Add(1)
		go func(idx int, c *websocket.Conn) {
			defer wg.Done()
			c.SetReadDeadline(time.Now().Add(1 * time.Second))
			var received Message
			err := c.ReadJSON(&received)
			if err != nil {
				t.Errorf("Connection %d failed to read message: %v", idx, err)
				return
			}
			if received.Type != "broadcast" {
				t.Errorf("Connection %d: Expected type 'broadcast', got '%s'", idx, received.Type)
			}
		}(i, conn)
	}

	wg.Wait()
}

func TestHub_DifferentUsers(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	// Создаем тестовый websocket сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := int64(1)
		if r.URL.Query().Get("user_id") == "2" {
			userID = 2
		}
		hub.HandleWebSocket(w, r, userID)
	}))
	defer server.Close()

	// Подключаемся как user 1
	wsURL1 := "ws" + server.URL[4:] + "?user_id=1"
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL1, nil)
	if err != nil {
		t.Fatalf("Failed to connect user 1: %v", err)
	}
	defer conn1.Close()

	// Подключаемся как user 2
	wsURL2 := "ws" + server.URL[4:] + "?user_id=2"
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL2, nil)
	if err != nil {
		t.Fatalf("Failed to connect user 2: %v", err)
	}
	defer conn2.Close()

	// Даем время на регистрацию
	time.Sleep(100 * time.Millisecond)

	// Отправляем сообщение только user 1
	message := &Message{
		Type:    "private",
		Channel: "test",
		Data:    map[string]interface{}{"test": "data"},
	}
	hub.Broadcast(1, message)

	// Проверяем, что только user 1 получил сообщение
	conn1.SetReadDeadline(time.Now().Add(1 * time.Second))
	var received1 Message
	err = conn1.ReadJSON(&received1)
	if err != nil {
		t.Fatalf("User 1 failed to read message: %v", err)
	}
	if received1.Type != "private" {
		t.Errorf("User 1: Expected type 'private', got '%s'", received1.Type)
	}

	// User 2 не должен получить сообщение
	conn2.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var received2 Message
	err = conn2.ReadJSON(&received2)
	if err == nil {
		t.Error("User 2 should not receive message for user 1")
	}
}

func TestHub_BroadcastChannelFull(t *testing.T) {
	hub := NewHub()
	// Создаем hub с маленьким каналом для теста
	hub.broadcast = make(chan *Message, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)

	// Заполняем канал
	hub.broadcast <- &Message{Type: "fill"}
	hub.broadcast <- &Message{Type: "fill"}

	// Попытка отправить еще одно сообщение (должно быть проигнорировано)
	message := &Message{
		Type:    "dropped",
		Channel: "test",
		Data:    map[string]interface{}{"test": "data"},
	}
	hub.Broadcast(1, message)

	// Проверяем, что сообщение не было добавлено (канал полон)
	select {
	case <-time.After(100 * time.Millisecond):
		// Ожидаемо - канал полон
	case msg := <-hub.broadcast:
		if msg.Type == "dropped" {
			t.Error("Message should be dropped when channel is full")
		}
	}
}

func TestHub_ShutdownClosesConnections(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())

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

	// Make sure connection is registered
	time.Sleep(50 * time.Millisecond)

	// Cancel the hub context -> Run should close underlying connections
	cancel()

	// Wait for hub to attempt shutdown
	time.Sleep(100 * time.Millisecond)

	// Attempt to read; should fail because server closed connection
	conn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("Expected connection to be closed after hub shutdown")
	}

	conn.Close()
}
