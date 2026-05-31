package bot

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ExtractTextFromPDF извлекает весь текст из PDF-файла по указанному пути
func ExtractTextFromPDF(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("не удалось открыть PDF: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	totalPages := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}

		content, err := p.GetPlainText(nil)
		if err != nil {
			// пропускаем страницу если не удалось прочитать
			continue
		}
		buf.WriteString(content)
		buf.WriteString("\n")
	}

	text := strings.TrimSpace(buf.String())
	if text == "" {
		return "", fmt.Errorf("PDF не содержит читаемого текста (возможно, это сканированное изображение)")
	}

	return text, nil
}