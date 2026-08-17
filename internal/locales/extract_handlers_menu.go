package locales

// ============================================================================
// Handlers — menu package strings (internal/bot/handlers/menu/*.go).
//
// Собраны пользовательские сообщения, тексты кнопок и подсказки callback-ов,
// извлечённые из пакета menu, чтобы централизовать их в locales. НЕ редактируйте
// существующие файлы locales ради этих констант — всё новое добавляется сюда.
// Примечание: строка «💎 Premium» уже существует как BtnPremium (keyboards.go)
// и переиспользуется как есть.
// ============================================================================

// ---------------------------------------------------------------------------
// Сообщения (Msg*)
// ---------------------------------------------------------------------------
const (
	// MsgPremiumSelectNewTariff - текст всплывающей подсказки (AnswerCallbackQuery)
	// при нажатии «🔄 Сменить тариф» у активного Premium.
	MsgPremiumSelectNewTariff = "Выберите новый тариф"

	// MsgPremiumChooseTariff - заголовок меню выбора тарифа Premium.
	MsgPremiumChooseTariff = "💎 Выберите тариф Premium:"

	// MsgPremiumTariffBullet - префикс строки «название тарифа» в меню выбора.
	MsgPremiumTariffBullet = "📌 "

	// MsgPremiumTariffPrice - префикс строки «цена тарифа» в меню выбора.
	MsgPremiumTariffPrice = "💰 "

	// MsgPremiumChooseBelow - подпись под списком тарифов.
	MsgPremiumChooseBelow = "Выберите тариф кнопкой ниже:"

	// MsgTariffNotFound - текст всплывающей подсказки, если тариф не найден.
	MsgTariffNotFound = "Тариф не найден"

	// MsgPaymentRedirecting - текст всплывающей подсказки при переходе к оплате.
	MsgPaymentRedirecting = "Перехожу к оплате..."

	// MsgPaymentCreateError - сообщение об ошибке создания платежа.
	MsgPaymentCreateError = "❌ Ошибка создания платежа. Попробуйте позже."

	// MsgPaymentInstructions - экран оплаты Premium (шаблон с подстановкой
	// названия тарифа, суммы и списка фич).
	MsgPaymentInstructions = "💳 Оплата Premium\n\n" +
		"📌 Тариф: %s\n" +
		"💰 Сумма: %s\n\n" +
		"🎁 Что входит:\n• %s\n\n" +
		"👇 Нажмите кнопку для оплаты:"

	// MsgPremiumActivateError - сообщение об ошибке активации Premium.
	MsgPremiumActivateError = "❌ Не удалось активировать Premium. Попробуйте позже."

	// MsgPremiumActivatedToast - текст всплывающей подсказки после активации.
	MsgPremiumActivatedToast = "✅ Premium активирован!"

	// MsgPremiumActivated - статус подтверждения (без галочки) в сообщении
	// об активации/смене тарифа.
	MsgPremiumActivated = "Premium активирован!"

	// MsgPremiumTariffChanged - статус подтверждения при смене тарифа.
	MsgPremiumTariffChanged = "Тариф изменён!"

	// MsgPremiumConfirmTariff - строка «Тариф: <name>» (шаблон).
	MsgPremiumConfirmTariff = "\n\nТариф: %s"

	// MsgPremiumConfirmExpiry - строка «Действует до: <date>» (шаблон).
	MsgPremiumConfirmExpiry = "\nДействует до: %s"

	// MsgPremiumConfirmFooter - завершающая часть сообщения после активации.
	MsgPremiumConfirmFooter = "\n\nТеперь вам доступна 💡 Сводка здоровья - откройте её кнопкой ниже или из главного меню."
)

// ---------------------------------------------------------------------------
// Кнопки (Btn*)
// ---------------------------------------------------------------------------
const (
	// BtnTariffPrefix - префикс кнопки выбора тарифа (в меню и в подтверждении).
	BtnTariffPrefix = "💎 "

	// BtnTariffSep - разделитель «название - цена» в кнопке выбора тарифа.
	BtnTariffSep = " - "

	// BtnPayWithPrice - кнопка оплаты с подставленной суммой (префикс).
	BtnPayWithPrice = "💳 Оплатить "

	// BtnPaidSimulation - кнопка симуляции оплаты.
	BtnPaidSimulation = "✅ Оплатил (симуляция)"

	// BtnOpen - кнопка открытия дашборда (WebApp).
	BtnOpen = "Открыть"
)
