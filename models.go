package main

type TestCase struct {
	Input    string `json:"input"`
	Expected string `json:"expected"`
	Code     string `json:"code"`
}

type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	StarterCode string     `json:"starter_code"`
	TestCases   []TestCase `json:"test_cases"`
}

func (t Task) Sanitize() Task {
	return Task{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		StarterCode: t.StarterCode,
		TestCases:   nil,
	}
}

// Снимок состояния кода
type CodeSnapshot struct {
	Timestamp int64  `json:"ts"`
	Code      string `json:"code"`
}

type UserStats struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	TotalScore    int             `json:"total_score"`
	CurrentTaskID string          `json:"current_task_id"`
	SolvedTasks   map[string]bool `json:"solved_tasks"`
	LastCode      string          `json:"last_code"`
	Status        string          `json:"status"`
	PasteContent  string          `json:"paste_content"`
	// ИСТОРИЯ ИЗМЕНЕНИЙ (Time Travel)
	History []CodeSnapshot `json:"history"`
}

type IncomingMessage struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type WsResponse struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type ClientMessage struct {
	client  *Client
	content IncomingMessage
}

type TestExecutionResult struct {
	TaskID  string `json:"task_id"`
	Stdout  string `json:"stdout"`
	Success bool   `json:"success"`
}
