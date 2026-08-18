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

// mergeIndicatorGroups объединяет несколько структурированных JSON-отчётов
// (по одному на файл) в один общий объект с массивами sections/categories.
func mergeIndicatorGroups(parts []string) string {
	sections := []json.RawMessage{}
	categories := []json.RawMessage{}
	for _, p := range parts {
		if strings.TrimSpace(p) == "" {
			continue
		}
		var doc struct {
			Sections   json.RawMessage `json:"sections"`
			Categories json.RawMessage `json:"categories"`
		}
		if err := json.Unmarshal([]byte(p), &doc); err != nil {
			continue
		}
		if len(doc.Sections) > 0 {
			var arr []json.RawMessage
			if json.Unmarshal(doc.Sections, &arr) == nil {
				sections = append(sections, arr...)
			}
		}
		if len(doc.Categories) > 0 {
			var arr []json.RawMessage
			if json.Unmarshal(doc.Categories, &arr) == nil {
				categories = append(categories, arr...)
			}
		}
	}
	if len(sections) == 0 && len(categories) == 0 {
		return ""
	}
	merged, err := json.Marshal(map[string]interface{}{
		"sections":   sections,
		"categories": categories,
	})
	if err != nil {
		return ""
	}
	return string(merged)
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
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
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
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
			return
		}
		combined := fileText + "\n\nДанные пациента и опросника об образе жизни:\n" + contextInfo
		// Сравнительный контекст: если ранее уже делали расширенный анализ -
		// подставляем предыдущий отчёт для СРАВНИТЕЛЬНОГО досье.
		if prevJSON, ok := monitoring.PreviousReportJSON(ctx, saver, chatID, "analysis"); ok {
			combined += locales.ComparisonContext(prevJSON, "analysis")
		}
		dossierJSON, derr := analysisService.HandleExtendedDossierJSON(ctx, combined)
		if derr != nil || dossierJSON == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
			return
		}
		renderAndSendDossier(ctx, b, stateManager, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, dossierJSON, appStorage, saver, webAppURL)
		return
	}

	result, err := analysisService.HandleAnalysisFromFileWithContext(
		ctx,
		fileData,
		file.MimeType,
		contextInfo,
	)

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	if err != nil {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
		return
	}

	// Пустой результат без явной ошибки - тоже «неверный ответ» ИИ.
	// Не отправляем пустоту и не ломаем состояние: даём пользователю
	// вернуться в главное меню.
	if strings.TrimSpace(result) == "" {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
		return
	}

	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   result,
	})

	// Дополнительно запрашиваем у ИИ структурированные показатели
	// (sections/categories с indicators), чтобы наполнить блоки «Сводки
	// здоровья» РЕАЛЬНЫМИ значениями обычного анализа (кровь/биохимия и т.п.).
	// Не критично: при ошибке сохраняем только текст (текущее поведение).
	indicatorsJSON, _ := analysisService.HandleAnalysisFromFileJSON(ctx, fileData, file.MimeType, contextInfo)

	// Сохраняем ОБЫЧНЫЙ анализ в «Мой профиль» (история пользователя),
	// чтобы он был доступен там вместе с прочими результатами. Вместе с
	// текстом сохраняем структурированные показатели для блоков дашборда.
	savePlainResult(ctx, saver, chatID, "analysis", locales.MsgUploadDefaultTitleAnalysis, result, indicatorsJSON)
	sendAnalysisComplete(ctx, b, stateManager, chatID)

	// Сообщаем, что результат сохранён в «Мой профиль», и даём
	// кнопку для мгновенного открытия.
	helpers.SendSavedToSummary(ctx, b, chatID, webAppURL)
}

// processMultipleFiles - обрабатывает несколько файлов.
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
	var collectedTexts []string
	collectedIndicators := []string{}

	for i, file := range files {
		fileData, err := file.readData()
		if err != nil {
			continue
		}

		res, err := analysisService.HandleAnalysisFromFileWithContext(
			ctx,
			fileData,
			file.MimeType,
			contextInfo,
		)
		if err == nil && res != "" {
			collectedTexts = append(collectedTexts, fmt.Sprintf("=== Данные из файла %d (%s) ===\n%s", i+1, file.FileName, res))
		}
		// Параллельно собираем структурированные показатели для дашборда.
		if ind, ierr := analysisService.HandleAnalysisFromFileJSON(ctx, fileData, file.MimeType, contextInfo); ierr == nil && ind != "" {
			collectedIndicators = append(collectedIndicators, ind)
		}
	}

	if len(collectedTexts) == 0 {
		sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
		return
	}

	combinedPayload := strings.Join(collectedTexts, "\n\n")

	if isExtended {
		// Расширенный анализ (несколько файлов) -> универсальное отчёт-досье.
		combined := strings.Join(collectedTexts, "\n\n")
		if combined == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
			return
		}
		combined += "\n\nДанные пациента и опросника об образе жизни:\n" + contextInfo
		// Сравнительный контекст: если ранее уже делали расширенный анализ -
		// подставляем предыдущий отчёт для СРАВНИТЕЛЬНОГО досье.
		if prevJSON, ok := monitoring.PreviousReportJSON(ctx, saver, chatID, "analysis"); ok {
			combined += locales.ComparisonContext(prevJSON, "analysis")
		}
		dossierJSON, derr := analysisService.HandleExtendedDossierJSON(ctx, combined)
		if derr != nil || dossierJSON == "" {
			sendAnalysisError(ctx, b, stateManager, chatID, loadingMsg, textMsg)
			return
		}
		renderAndSendDossier(ctx, b, stateManager, reportRenderer, pdfConverter, chatID, loadingMsg, textMsg, dossierJSON, appStorage, saver, webAppURL)
		return
	}

	deleteLoadingMessages(ctx, b, chatID, loadingMsg, textMsg)

	finalResult := fmt.Sprintf(locales.MsgUploadResultFiles, len(files), combinedPayload)
	_, _ = b.SendMessage(ctx, &tgbot.SendMessageParams{
		ChatID: chatID,
		Text:   finalResult,
	})

	// Структурированные показатели (объединяем по всем файлам) для блоков
	// «Мой профиль».
	indicatorsJSON := mergeIndicatorGroups(collectedIndicators)

	// Сохраняем ОБЫЧНЫЙ анализ (несколько файлов) в «Мой профиль».
	savePlainResult(ctx, saver, chatID, "analysis", locales.MsgUploadDefaultTitleAnalysis, finalResult, indicatorsJSON)
	sendAnalysisComplete(ctx, b, stateManager, chatID)

	// Сообщаем, что результат сохранён в «Мой профиль», и даём
	// кнопку для мгновенного открытия.
	helpers.SendSavedToSummary(ctx, b, chatID, webAppURL)
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
func sendAnalysisError(ctx context.Context, b *tgbot.Bot, stateManager states.StateManager, chatID int64, loadingMsg, textMsg *models.Message) {
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
// дополнительной информации (extra). Используется для расширенного анализа/
// досье: в extra передаём запроса на сравнение с предыдущим отчётом и
// напоминание, что динамику видно в «Мой профиль».
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

// buildReportNote собирает текст доп. блока для выдачи расширенного отчёта:
// напоминание о сравнении повторных отчётов + краткое сравнение (summary),
// если ИИ сформировал сравнительный отчёт. jsonResult - JSON отчёта.
func buildReportNote(jsonResult string) string {
	parts := []string{locales.MsgReportProgressNote}
	if s := monitoring.ParseComparisonSummary(jsonResult); s != "" {
		parts = append(parts, "📈 Сравнение с предыдущим отчётом: "+s)
	}
	return strings.Join(parts, "\n\n")
}
