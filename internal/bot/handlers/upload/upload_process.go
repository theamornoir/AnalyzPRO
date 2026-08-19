package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	tgbot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/theamornoir/analyzpro/internal/bot/handlers/helpers"
	"github.com/theamornoir/analyzpro/internal/bot/keyboards"
	"github.com/theamornoir/analyzpro/internal/bot/states"
	"github.com/theamornoir/analyzpro/internal/locales"
	"github.com/theamornoir/analyzpro/internal/monitoring"
	"github.com/theamornoir/analyzpro/internal/report"
	"github.com/theamornoir/analyzpro/internal/report/pdfservice"
	"github.com/theamornoir/analyzpro/internal/service"
	"github.com/theamornoir/analyzpro/internal/storage"
)

// savePlainResult сохраняет «неструктурированный» результат (обычный анализ
// текстом или базовый Bioscan) в историю пользователя как запись типа
// entryType, чтобы он появился в «Мой профиль» вместе с прочими
// результатами. Формирует аккуратный HTML-документ (ReportHTML), чтобы
// кнопка «📄 PDF» в Сводке открывала именно этот результат без ошибки рендера.
//
// indicatorsJSON - опциональный структурированный JSON отчёта (sections/
// categories с indicators), полученный от ИИ. Если непустой и валидный - его
// группы показателей вливаются в JsonData записи, чтобы дашборд «Мой
// профиль» мог строить блоки (кровь/биохимия и т.п.) и заполнять карточки
// крови/питания/активности из РЕАЛЬНЫХ показателей обычного анализа.
func savePlainResult(ctx context.Context, saver monitoring.Repository, chatID int64, entryType, title, note, indicatorsJSON string) {
	if saver == nil || strings.TrimSpace(note) == "" {
		return
	}
	// Итоговый JSON записи: человекочитаемый текст (note) + структурированные
	// показатели (sections/categories) для блоков дашборда.
	record := map[string]interface{}{
		"title": title,
		"note":  note,
	}
	if groups := extractIndicatorGroups(indicatorsJSON); groups != nil {
		if sections, ok := groups["sections"]; ok {
			record["sections"] = sections
		}
		if categories, ok := groups["categories"]; ok {
			record["categories"] = categories
		}
	}
	payload, err := json.Marshal(record)
	if err != nil {
		// На крайний случай - хотя бы текст.
		payload = []byte(fmt.Sprintf(`{"title":%q,"note":%q}`, title, note))
	}
	entry := &monitoring.HistoryEntry{
		TelegramID: chatID,
		Type:       entryType,
		Title:      title,
		Date:       time.Now(),
		JsonData:   string(payload),
		ReportHTML: helpers.PlainResultHTML(title, note),
	}
	if err := saver.SaveResult(ctx, entry); err != nil {
		log.Printf("[UPLOAD] не удалось сохранить %s chatID=%d: %v", entryType, chatID, err)
	}
}

// extractIndicatorGroups извлекает массивы sections/categories из JSON
// структурированного отчёта анализа (возвращается ИИ). Возвращает nil, если
// валидного JSON нет или в обоих массивах нет элементов.
// stripJSONFences убирает markdown-ограждения (```json ... ```) и случайный
// текст вокруг JSON, которые модель иногда добавляет даже при явном запросе
// «строго JSON». Без этого json.Unmarshal падает, и структурированные
// показатели не попадают в «Мой профиль» (блоки не строятся даже при
// активной Premium).
func stripJSONFences(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "```json", "")
	s = strings.ReplaceAll(s, "```", "")
	s = strings.TrimSpace(s)
	// Отбрасываем возможный текст до первой «{» и после последней «}».
	if start := strings.Index(s, "{"); start > 0 {
		s = s[start:]
	}
	if end := strings.LastIndex(s, "}"); end >= 0 && end < len(s)-1 {
		s = s[:end+1]
	}
	return strings.TrimSpace(s)
}

func extractIndicatorGroups(indicatorsJSON string) map[string]interface{} {
	s := stripJSONFences(indicatorsJSON)
	if s == "" {
		return nil
	}
	var doc struct {
		Sections   json.RawMessage `json:"sections"`
		Categories json.RawMessage `json:"categories"`
	}
	if err := json.Unmarshal([]byte(s), &doc); err != nil {
		return nil
	}
	out := map[string]interface{}{}
	if len(doc.Sections) > 0 && string(doc.Sections) != "null" {
		var arr []json.RawMessage
		if json.Unmarshal(doc.Sections, &arr) == nil && len(arr) > 0 {
			out["sections"] = arr
		}
	}
	if len(doc.Categories) > 0 && string(doc.Categories) != "null" {
		var arr []json.RawMessage
		if json.Unmarshal(doc.Categories, &arr) == nil && len(arr) > 0 {
			out["categories"] = arr
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// processSingleFile - обрабатывает один файл.
func processSingleFile(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	file UploadedFile,
	isExtended bool,
	contextInfo string,
	appStorage *storage.Storage,
	saver monitoring.Repository,
	webAppURL string,
) {
	fileData, err := file.readData()
	if err != nil {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, err)
		return
	}

	if isExtended {
		// Расширенный анализ -> универсальное отчёт-досье здоровья.
		fileText, ferr := analysisService.HandleAnalysisFromFileWithContext(
			ctx,
			fileData,
			file.MimeType,
			contextInfo,
		)
		if ferr != nil || fileText == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("extended single-file analysis failed: %w", ferr))
			return
		}
		combined := fileText + "\n\nДанные пациента и опросника об образе жизни:\n" + contextInfo
		dossierJSON, derr := analysisService.HandleExtendedDossierJSON(ctx, combined)
		if derr != nil || dossierJSON == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("extended dossier JSON failed: %w", derr))
			return
		}
		renderAndSendDossier(ctx, b, stateManager, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, dossierJSON, appStorage, saver, webAppURL)
		return
	}

	// Обычный анализ: ОДИН структурированный JSON-вызов ИИ. По нему
	// детерминированно рендерим чат-текст (гарантия формата, без зависимости
	// от «настроения» LLM, как в Bioscan) и тот же JSON сохраняем в «Мой
	// профиль» для блоков дашборда. Два старых вызова (текст + JSON)
	// объединены в один.
	analysisJSON, err := analysisService.HandleAnalysisFromFileJSON(
		ctx,
		fileData,
		file.MimeType,
		contextInfo,
	)
	if err != nil || strings.TrimSpace(analysisJSON) == "" {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("analysis JSON failed: %w", err))
		return
	}
	parsed, perr := report.ParseAdaptiveReportJSON(analysisJSON)
	if perr != nil {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("analysis JSON parse failed: %w", perr))
		return
	}
	result := report.RenderAnalysisPlainText(parsed)
	if strings.TrimSpace(result) == "" {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("rendered empty analysis for single file"))
		return
	}

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	sendLongMessagePlain(ctx, b, chatID, result)

	// Сохраняем ОБЫЧНЫЙ анализ в «Мой профиль» (история пользователя),
	// чтобы он был доступен там вместе с прочими результатами. Тот же
	// структурированный JSON наполняет блоки дашборда РЕАЛЬНЫМИ
	// показателями обычного анализа (кровь/биохимия и т.п.).
	savePlainResult(ctx, saver, chatID, "analysis", locales.MsgUploadDefaultTitleAnalysis, result, analysisJSON)
	sendAnalysisComplete(ctx, b, stateManager, chatID)

	// Сообщаем, что результат сохранён в «Мой профиль», и даём
	// кнопку для мгновенного открытия.
	helpers.SendSavedToSummary(ctx, b, chatID, webAppURL)
}

// processMultipleFiles - обрабатывает несколько файлов ЕДИНЫМ мультимодальным
// запросом. Все вложения (фото/PDF) передаются в Gemini в ОДНО сообщение
// вместе с промптом (как в Bioscan PRO) - это исключает потерю данных между
// файлами и позволяет ИИ видеть связь между несколькими анализами сразу.
func processMultipleFiles(
	ctx context.Context,
	b *tgbot.Bot,
	stateManager states.StateManager,
	analysisService service.AnalysisService,
	reportRenderer *report.Renderer,
	pdfConverter pdfservice.Converter,
	chatID int64,
	loadingMsg *models.Message,
	textMsg *models.Message,
	files []UploadedFile,
	isExtended bool,
	contextInfo string,
	appStorage *storage.Storage,
	saver monitoring.Repository,
	webAppURL string,
) {
	// Собираем байты и MIME-типы всех файлов для единого запроса.
	var filesData [][]byte
	var mimeTypes []string
	for _, file := range files {
		fileData, err := file.readData()
		if err != nil {
			continue
		}
		filesData = append(filesData, fileData)
		mimeTypes = append(mimeTypes, file.MimeType)
	}

	if len(filesData) == 0 {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("no file data read from any uploaded file"))
		return
	}

	if isExtended {
		// Расширенный анализ (несколько файлов) -> универсальное отчёт-досье.
		// Один мультимодальный текстовый запрос всех вложений сразу
		// (analysisText учитывает ВСЕ файлы совместно).
		analysisText, err := analysisService.HandleAnalysisFromFilesWithContext(
			ctx,
			filesData,
			mimeTypes,
			contextInfo,
		)
		if err != nil || strings.TrimSpace(analysisText) == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("multi-file analysis failed: %w", err))
			return
		}
		combined := analysisText + "\n\nДанные пациента и опросника об образе жизни:\n" + contextInfo
		dossierJSON, derr := analysisService.HandleExtendedDossierJSON(ctx, combined)
		if derr != nil || dossierJSON == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("multi-file extended dossier JSON failed: %w", derr))
			return
		}
		renderAndSendDossier(ctx, b, stateManager, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, dossierJSON, appStorage, saver, webAppURL)
		return
	}

	// БАЗОВЫЙ анализ: ОДИН структурированный JSON-вызов ИИ всех файлов сразу
	// -> детерминированный чат-текст (гарантия формата, без зависимости от
	// «настроения» LLM, как в Bioscan) + блоки дашборда «Мой профиль».
	analysisJSON, err := analysisService.HandleAnalysisFromFilesJSON(
		ctx,
		filesData,
		mimeTypes,
		contextInfo,
	)
	if err != nil || strings.TrimSpace(analysisJSON) == "" {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("multi-file analysis JSON failed: %w", err))
		return
	}
	parsed, perr := report.ParseAdaptiveReportJSON(analysisJSON)
	if perr != nil {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("multi-file analysis JSON parse failed: %w", perr))
		return
	}
	rendered := report.RenderAnalysisPlainText(parsed)
	if strings.TrimSpace(rendered) == "" {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg, fmt.Errorf("rendered empty multi-file analysis"))
		return
	}

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	finalResult := fmt.Sprintf(locales.MsgUploadResultFiles, len(files), rendered)
	sendLongMessagePlain(ctx, b, chatID, finalResult)

	// Сохраняем ОБЫЧНЫЙ анализ (несколько файлов) в «Мой профиль».
	// Тот же структурированный JSON наполняет блоки дашборда РЕАЛЬНЫМИ
	// показателями обычного анализа.
	savePlainResult(ctx, saver, chatID, "analysis", locales.MsgUploadDefaultTitleAnalysis, finalResult, analysisJSON)
	sendAnalysisComplete(ctx, b, stateManager, chatID)

	// Сообщаем, что результат сохранён в «Мой профиль», и даём
	// кнопку для мгновенного открытия.
	helpers.SendSavedToSummary(ctx, b, chatID, webAppURL)
}

// sendLongMessagePlain отправляет текст, разбивая его на куски <= 4000
// символов по границам строк, чтобы не упереться в лимит Telegram (4096).
// Клавиатура не крепится - меню присылается отдельным сообщением после
// сохранения результата. Без этого длинные отчёты ИИ (4000-6000 выходных
// токенов на кириллице - это ~6-10k символов) не доставлялись бы в чат
// (Telegram вернул бы 400), хотя и сохранялись бы в «Мой профиль».
func sendLongMessagePlain(ctx context.Context, b *tgbot.Bot, chatID int64, text string) {
	const maxChunk = 4000
	runes := []rune(text)
	n := len(runes)
	if n <= maxChunk {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: text})
		return
	}

	chunks := []string{}
	for start := 0; start < n; {
		end := start + maxChunk
		if end > n {
			end = n
		}
		chunk := string(runes[start:end])
		if end < n {
			if idx := strings.LastIndex(chunk, "\n"); idx > 0 {
				end = start + idx + 1
				chunk = string(runes[start:end])
			}
		}
		chunks = append(chunks, chunk)
		start = end
	}

	for _, chunk := range chunks {
		_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{ChatID: chatID, Text: chunk})
	}
}

// deleteLoadingMessages - удаляет сообщения о загрузке.
func deleteLoadingMessages(ctx context.Context, b *tgbot.Bot, chatID int64, loadingMsg, textMsg *models.Message) {
	if loadingMsg != nil {
		helpers.DeleteMessage(ctx, b, chatID, loadingMsg.ID)
	}
	if textMsg != nil {
		helpers.DeleteMessage(ctx, b, chatID, textMsg.ID)
	}
}

// sendAnalysisError - отправляет сообщение об ошибке обработки анализа
// вместе с главным меню и сбрасывает состояние. Это гарантирует, что после
// неудачной обработки пользователь не «зависает» без клавиатуры и может
// вернуться в меню. Раньше ошибка отправлялась через helpers.SendError без
// клавиатуры и без сброса состояния - из-за этого пропадало меню и не было
// пути назад. loadingMsg/textMsg безопасно обрабатываются, если nil.
//
// err - реальная причина сбоя (ошибка Gemini/сети/чтения файла). Раньше она
// «проглатывалась»: в логах оставался только generic-текст для юзера, из-за
// чего было непонятно, почему анализ упал (например, 404 по снятой с
// поддержки модели или пустой ключ API). Теперь причина логируется.
func sendAnalysisError(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, loadingMsg, textMsg *models.Message, err error) {
	if err != nil {
		log.Printf("❌ [UPLOAD] анализ не удалось обработать (chatID=%d): %v", chatID, err)
	}
	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)
	stateManager.Reset(chatID)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        locales.MsgTextProcessingError,
		ReplyMarkup: keyboards.MainMenu(),
	})
}

// sendAnalysisComplete - отправляет сообщение о завершении анализа и сбрасывает состояние.
func sendAnalysisComplete(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64) {
	sendAnalysisCompleteNote(ctx, b, stateManager, chatID, "")
}

// sendAnalysisCompleteNote - как sendAnalysisComplete, но дописывает блок
// дополнительной информации (extra). Используется, чтобы напомнить
// пользователю, что результат сохранён в «Мой профиль».
func sendAnalysisCompleteNote(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, extra string) {
	stateManager.Reset(chatID)
	text := locales.MsgAnalysisComplete
	if strings.TrimSpace(extra) != "" {
		text += "\n\n" + extra
	}
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: keyboards.MainMenu(),
		ParseMode:   "HTML",
	})
}
