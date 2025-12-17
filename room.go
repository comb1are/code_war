package main

import (
	"sync"
	"time"
)

type Room struct {
	ID        string
	clients   map[*Client]bool
	userStats map[string]*UserStats
	AllTasks  []Task
	mu        sync.RWMutex

	register   chan *Client
	unregister chan *Client
	broadcast  chan *ClientMessage
}

func newRoom(id string) *Room {
	tasks := []Task{
		{
			ID: "1", Title: "Привет, Python", Description: "Напишите функцию hello(), которая возвращает строку 'Hello World'",
			StarterCode: "def hello():\n    return ''",
			TestCases: []TestCase{
				{Code: "assert hello() == 'Hello World', 'Должно быть Hello World'"},
			},
		},
	}

	return &Room{
		ID:         id,
		clients:    make(map[*Client]bool),
		userStats:  make(map[string]*UserStats),
		AllTasks:   tasks,
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *ClientMessage),
	}
}

func (r *Room) SaveTask(t Task) {
	r.mu.Lock()
	found := false
	for i, existing := range r.AllTasks {
		if existing.ID == t.ID {
			r.AllTasks[i] = t
			found = true
			break
		}
	}
	if !found {
		r.AllTasks = append(r.AllTasks, t)
	}
	r.mu.Unlock()
	r.broadcastTaskList()
}

func (r *Room) run() {
	for {
		select {
		case client := <-r.register:
			r.mu.Lock()
			r.clients[client] = true
			if _, exists := r.userStats[client.ID]; !exists {
				r.userStats[client.ID] = &UserStats{
					ID:          client.ID,
					Name:        client.Name,
					SolvedTasks: make(map[string]bool),
					Status:      "Idle",
					History:     make([]CodeSnapshot, 0), // Инициализация
				}
			} else {
				r.userStats[client.ID].Name = client.Name
			}
			r.mu.Unlock()
			r.sendTaskList(client)
			r.broadcastAdminStats()

		case client := <-r.unregister:
			r.mu.Lock()
			delete(r.clients, client)
			r.mu.Unlock()
			r.broadcastAdminStats()

		case msg := <-r.broadcast:
			r.handleMessage(msg)
		}
	}
}

func (r *Room) handleMessage(msg *ClientMessage) {
	switch msg.content.Type {
	case "submit":
		r.processSubmission(msg.client, msg.content.Payload)
	case "code_update":
		r.updateUserCode(msg.client.ID, msg.content.Payload)
	case "select_task":
		r.handleTaskSelection(msg.client, msg.content.Payload)
	case "cheat_warning":
		r.flagCheater(msg.client.ID, msg.content.Payload)
	}
}

func (r *Room) flagCheater(userID, content string) {
	r.mu.Lock()
	if stats, ok := r.userStats[userID]; ok {
		stats.Status = "⚠️ COPIED!"
		stats.PasteContent = content
	}
	r.mu.Unlock()
	r.broadcastAdminStats()
}

func (r *Room) handleTaskSelection(client *Client, taskID string) {
	r.mu.Lock()
	if stats, ok := r.userStats[client.ID]; ok {
		stats.CurrentTaskID = taskID
		if stats.Status == "⚠️ COPIED!" {
			stats.Status = "Idle"
		}
	}
	r.mu.Unlock()
	r.broadcastAdminStats()
}

func (r *Room) updateUserCode(userID, code string) {
	r.mu.Lock()
	if stats, ok := r.userStats[userID]; ok {
		stats.LastCode = code
		if stats.Status != "Solved" && stats.Status != "⚠️ COPIED!" {
			stats.Status = "Typing"
		}

		// Добавляем снепшот только если код изменился
		if len(stats.History) == 0 || stats.History[len(stats.History)-1].Code != code {
			stats.History = append(stats.History, CodeSnapshot{
				Timestamp: time.Now().UnixMilli(),
				Code:      code,
			})
		}
	}
	r.mu.Unlock()

	// ВАЖНО: Включаем рассылку обновлений при печати, чтобы Replay обновлялся в реальном времени
	go r.broadcastAdminStats()
}

func (r *Room) processSubmission(player *Client, code string) {
	r.mu.RLock()
	stats := r.userStats[player.ID]
	taskID := stats.CurrentTaskID
	var currentTask Task
	found := false
	for _, t := range r.AllTasks {
		if t.ID == taskID {
			currentTask = t
			found = true
			break
		}
	}
	r.mu.RUnlock()

	if !found {
		return
	}

	r.mu.Lock()
	if stats.Status != "⚠️ COPIED!" {
		stats.Status = "Testing"
	}
	stats.LastCode = code

	// Финальный снепшот
	stats.History = append(stats.History, CodeSnapshot{
		Timestamp: time.Now().UnixMilli(),
		Code:      code,
	})

	r.mu.Unlock()
	r.broadcastAdminStats()

	go func() {
		logs, _, success := runGoCode(code, currentTask)
		player.send <- WsResponse{
			Type: "test_result",
			Payload: TestExecutionResult{
				TaskID:  taskID,
				Stdout:  logs,
				Success: success,
			},
		}

		r.mu.Lock()
		if success {
			stats.Status = "Solved"
			if !stats.SolvedTasks[taskID] {
				stats.SolvedTasks[taskID] = true
				stats.TotalScore += 1
			}
		} else {
			if stats.Status != "⚠️ COPIED!" {
				stats.Status = "Failed"
			}
		}
		r.mu.Unlock()
		r.broadcastAdminStats()
	}()
}

func (r *Room) sendTaskList(client *Client) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var sanitized []Task
	for _, t := range r.AllTasks {
		sanitized = append(sanitized, t.Sanitize())
	}
	client.send <- WsResponse{Type: "task_list", Payload: sanitized}
	if stats, ok := r.userStats[client.ID]; ok {
		client.send <- WsResponse{Type: "user_stats", Payload: stats}
	}
}

func (r *Room) broadcastTaskList() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var sanitized []Task
	for _, t := range r.AllTasks {
		sanitized = append(sanitized, t.Sanitize())
	}
	msg := WsResponse{Type: "task_list", Payload: sanitized}
	for client := range r.clients {
		select {
		case client.send <- msg:
		default:
			close(client.send)
			delete(r.clients, client)
		}
	}
}

func (r *Room) broadcastAdminStats() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var statsList []*UserStats
	for _, s := range r.userStats {
		statsList = append(statsList, s)
	}
	msg := WsResponse{Type: "global_stats", Payload: statsList}
	for client := range r.clients {
		select {
		case client.send <- msg:
		default:
			close(client.send)
			delete(r.clients, client)
		}
	}
}
