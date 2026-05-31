package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const deepseekAPI = "https://api.deepseek.com/chat/completions"

const analyzePrompt = `Ты — HR-специалист. Оцени резюме кандидата на вакансию "%s".

Требования к вакансии:
%s

Резюме:
---
%s
---

Ответь СТРОГО в таком формате, без лишних слов:

Честная строгая оценка: X/100
Статус: ПРИНЯТ / ПРИВЛЕК ВНИМАНИЕ / ОТКЛОНЁН

Оценка 70+ → ПРИНЯТ, 50–69 → НА РАССМОТРЕНИЕ, ниже 50 → ОТКЛОНЁН.`


type dsMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type dsRequest struct {
	Model    string      `json:"model"`
	Messages []dsMessage `json:"messages"`
	Stream   bool        `json:"stream"`
}

type dsResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// AnalyzeCV отправляет извлечённый текст резюме в DeepSeek и возвращает анализ
func AnalyzeCV(ctx context.Context, vacancyTitle, vacancyDescription, cvText string) (string, error) {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("DEEPSEEK_API_KEY не задан")
	}

	// Логируем первые 300 символов извлечённого текста — чтобы убедиться что это текст, не бинарник
	preview := cvText
	if len(preview) > 300 {
		preview = preview[:300] + "..."
	}
	log.Printf("[PDF текст — превью]:\n%s", preview)

	// Обрезаем если слишком длинное
	if len(cvText) > 15000 {
		cvText = cvText[:15000] + "\n...[текст обрезан]"
	}

	prompt := fmt.Sprintf(analyzePrompt, vacancyTitle, vacancyDescription, cvText)

	body, err := json.Marshal(dsRequest{
		Model: "deepseek-chat",
		Messages: []dsMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	// Retry 3 раза при 503/429
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		result, err := doRequest(ctx, apiKey, body)
		if err == nil {
			return result, nil
		}
		lastErr = err
		log.Printf("[DeepSeek] попытка %d/3 не удалась: %v", attempt, err)
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(attempt*3) * time.Second):
		}
	}
	return "", lastErr
}

func doRequest(ctx context.Context, apiKey string, body []byte) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deepseekAPI, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("DeepSeek вернул %d: %s", resp.StatusCode, string(raw))
	}

	var ds dsResponse
	if err := json.Unmarshal(raw, &ds); err != nil {
		return "", fmt.Errorf("unmarshal: %w", err)
	}
	if ds.Error != nil {
		return "", fmt.Errorf("DeepSeek error: %s", ds.Error.Message)
	}
	if len(ds.Choices) == 0 {
		return "", fmt.Errorf("DeepSeek вернул пустой ответ")
	}

	return strings.TrimSpace(ds.Choices[0].Message.Content), nil
}