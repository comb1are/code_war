package main

import (
	"encoding/json"
	"io"
	"os/exec"
	"strings"
)

type SandboxRequest struct {
	Code  string     `json:"code"`
	Tests []TestCase `json:"tests"`
}

type SandboxResponse struct {
	Stdout  string `json:"stdout"`
	Success bool   `json:"success"`
	Passed  int    `json:"passed"`
}

func runGoCode(userCode string, task Task) (string, int, bool) {
	// 1. Формируем JSON
	req := SandboxRequest{
		Code:  userCode,
		Tests: task.TestCases,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return "JSON Error", 0, false
	}

	// 2. Готовим команду: python3 sandbox.py (без аргументов)
	// Для Windows возможно придется заменить "python3" на "python"
	cmd := exec.Command("python3", "sandbox.py")

	// Создаем трубу (Pipe) для передачи данных в stdin процесса Python
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "System Error: Pipe failed", 0, false
	}

	// Запускаем горутину, которая запишет JSON в Python и закроет трубу
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, string(reqBytes))
	}()

	// 3. Запускаем и ждем вывода
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		// Часто бывает, что python3 не найден.
		outputStr := string(outputBytes)
		if strings.Contains(err.Error(), "executable file not found") {
			return "Error: Python interpreter not found. Try changing 'python3' to 'python' in runner.go", 0, false
		}
		return "Sandbox System Error:\n" + outputStr, 0, false
	}

	// 4. Парсим ответ
	var resp SandboxResponse
	if err := json.Unmarshal(outputBytes, &resp); err != nil {
		// Если Python упал и вывел не JSON, а traceback
		return "Sandbox Crash:\n" + string(outputBytes), 0, false
	}

	return resp.Stdout, resp.Passed, resp.Success
}
