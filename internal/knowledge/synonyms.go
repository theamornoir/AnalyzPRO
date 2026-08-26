package knowledge

import (
	"regexp"
	"strings"
)

// digitPunctRE удаляет цифры и пунктуацию при разбиении текста на
// токены для точного сопоставления коротких синонимов показателей.
var digitPunctRE = regexp.MustCompile(`[\d\p{P}]+`)

// indicatorAliases сопоставляет частые варианты написания показателей в
// лабораторных бланках с кодами kn_reference_ranges. Ключи — нормализованные
// (без ё/дефисов/пробелов, нижний регистр, латиница по возможности).
// Дополняет токенное сопоставление MatchIndicator и покрывает сокращения
// (ЛПНП, АЛТ, ТТГ, HbA1c), которые не являются подстрокой канонического имени.
//
// При добавлении нового показателя в seed.sql — добавьте сюда его
// распространённые синонимы, чтобы он надёжно матчился из PDF/фото.
var indicatorAliases = map[string]string{
	// Глюкоза / углеводный обмен
	"глюкоза":        "GLU",
	"глюкозанатощак": "GLU",
	"сахар":          "GLU",
	"сахаркрови":     "GLU",
	"glucose":        "GLU",
	"гликированныйгемоглобин": "GHB",
	"гликированный":           "GHB",
	"ghb":                     "GHB",
	"hba1c":                   "GHB",
	"glycatedhemoglobin":      "GHB",

	// Липидограмма
	"холестерин":      "CHOL",
	"холестеринобщий": "CHOL",
	"общийхолестерин": "CHOL",
	"chol":            "CHOL",
	"cholesterol":     "CHOL",
	"лпнп":            "LDL",
	"холестеринлпнп":  "LDL",
	"ldl":             "LDL",
	"лпвп":            "HDL",
	"холестеринлпвп":  "HDL",
	"hdl":             "HDL",
	"триглицериды":    "TG",
	"тг":              "TG",
	"tg":              "TG",
	"triglycerides":   "TG",

	// Гематология
	"гемоглобин": "HGB",
	"hgb":        "HGB",
	"hemoglobin": "HGB",
	"эритроциты": "RBC",
	"rbc":        "RBC",
	"лейкоциты":  "WBC",
	"лейко":      "WBC",
	"wbc":        "WBC",
	"тромбоциты": "PLT",
	"plt":        "PLT",
	"гематокрит": "HCT",
	"гт":         "HCT",
	"hct":        "HCT",
	"соэ":        "ESR",
	"соскорость": "ESR",
	"esr":        "ESR",

	// Биохимия
	"креатинин":      "CREA",
	"crea":           "CREA",
	"creatinine":     "CREA",
	"мочевина":       "UREA",
	"urea":           "UREA",
	"мочеваякислота": "UA",
	"ua":             "UA",
	"uricacid":       "UA",
	"алт":            "ALT",
	"аланинаминотрансфераза": "ALT",
	"alt": "ALT",
	"аст": "AST",
	"аспартатаминотрансфераза": "AST",
	"ast":            "AST",
	"билирубин":      "TBIL",
	"билирубинобщий": "TBIL",
	"tbil":           "TBIL",
	"bilirubin":      "TBIL",

	// Гормоны / микроэлементы / витамины
	"ттг":                "TSH",
	"тиреотропныйгормон": "TSH",
	"tsh":                "TSH",
	"т4свободный":        "T4F",
	"свободныйтироксин":  "T4F",
	"t4f":                "T4F",
	"т3свободный":        "T3F",
	"t3f":                "T3F",
	"ферритин":           "FERR",
	"ferr":               "FERR",
	"ferritin":           "FERR",
	"витаминd":           "VITD",
	"витаминd25oh":       "VITD",
	"vitd":               "VITD",
	"25ohd":              "VITD",
	"железо":             "FE",
	"fe":                 "FE",
	"iron":               "FE",
	"кальций":            "CA",
	"кальцийобщий":       "CA",
	"ca":                 "CA",
	"calcium":            "CA",
	"натрий":             "NA",
	"na":                 "NA",
	"sodium":             "NA",
	"калий":              "K",
	"k":                  "K",
	"potassium":          "K",
}

// matchAliasByCode ищет код показателя по синонимам в нормализованном
// (без пробелов/пунктуации) тексте. Возвращает код и true, если найдено.
func matchAliasByCode(normalizedText string) (string, bool) {
	// Короткие синонимы (<=4 символов: лпнп, алт, ттг, тг, hba1c...) могут
	// ложно срабатывать внутри длинных слов, поэтому требуем их как
	// отдельный «токен» (граничное вхождение). Длинные — допускаем как
	// подстроку (например, гликированныйгемоглобин в слитном тексте).
	for alias, code := range indicatorAliases {
		if len([]rune(alias)) <= 4 {
			if _, ok := tokenSet(normalizedText)[alias]; ok {
				return code, true
			}
			continue
		}
		if strings.Contains(normalizedText, alias) {
			return code, true
		}
	}
	return "", false
}

// tokenSet разбивает нормализованный текст на уникальные токены (без цифр/
// пунктуации), пригодные для точного сопоставления коротких синонимов.
func tokenSet(s string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, t := range strings.Fields(digitPunctRE.ReplaceAllString(s, " ")) {
		set[t] = struct{}{}
	}
	return set
}
