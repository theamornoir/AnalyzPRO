package locales

// String-literal extraction results for the onboarding handler package
// (scope: internal/bot/handlers/onboarding/*.go, non-test).
//
// The onboarding package already centralizes every user-facing message and
// button label inside internal/locales/onboarding.go and reuses those
// constants directly. No new constants were required for this scope and the
// source file needed no inline-literal replacements. The reused constants are:
//
//   - locales.MsgOnboardingIntro       (1-е сообщение: единое описание функционала)
//   - locales.MsgOnboardingAgreement   (2-е сообщение: краткое согласие + ссылка)
//   - locales.MsgOnboardingDone        (post-accept welcome)
//   - locales.MsgResetDone             (admin reset reply)
//   - locales.BtnOnboardingAgreement   (button "📝 Согласие")
//   - locales.BtnOnboardingAccept      (button "✅ Принять")
//
// NOTE: онбординг сокращён до ДВУХ сообщений (интро + согласие). Старые
// константы слайдера (MsgOnboardingStep1..8, BtnOnboardingNext) и
// UserAgreementText удалены; полный текст согласия теперь размещён на
// внешнем ресурсе и доступен по короткой ссылке, подставляемой в runtime
// (onboarding.SendAgreement добавляет строку с внешней ссылкой).
//
// After reusing the constants above, the only remaining inline string literals
// in the onboarding source are NOT eligible for extraction (kept inline on
// purpose, matching project conventions):
//
//   - "onboarding_agreement" -> Telegram callback-data string (do-not-extract).
//   - "onboarding_accept"    -> Telegram callback-data string (do-not-extract).
//   - "Markdown" (x2)        -> Telegram ParseMode enum value. The codebase keeps
//                               parse modes ("Markdown"/"HTML") inline in every
//                               handler (router, bioscan, agreement, upload), so it
//                               is treated as an identifier/enum string and left
//                               inline (do-not-extract).
//
// As a result, this file intentionally contains no new exported constants; it
// documents the analysis and confirms the onboarding scope required no further
// extraction or source edits.
