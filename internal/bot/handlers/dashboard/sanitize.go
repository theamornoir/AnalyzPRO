package dashboard

import "regexp"

// sanitizeReportHTML - последняя линия защиты от stored XSS. Отчёты
// (ReportHTML/JsonData) формируются ИИ и через prompt-injection могут
// содержать инъектированную разметку. Убираем <script>/<style>/<iframe>/
// <object>/<embed>, обработчики событий on*= и javascript:/vbscript:/data:
// в href/src. Наши шаблоны используют только style-атрибуты и inline-SVG с
// числовыми значениями, поэтому легитимная вёрстка сохраняется.
var (
	reScript    = regexp.MustCompile(`(?is)<script.*?</script>`)
	reStyle     = regexp.MustCompile(`(?is)<style.*?</style>`)
	reIframe    = regexp.MustCompile(`(?is)<iframe.*?</iframe>`)
	reObject    = regexp.MustCompile(`(?is)<object.*?</object>`)
	reEmbed     = regexp.MustCompile(`(?is)<embed\b.*?>`)
	reEvent     = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	reJSProto   = regexp.MustCompile(`(?i)(href|src)\s*=\s*("|')\s*javascript:`)
	reDangerURI = regexp.MustCompile(`(?i)(href|src)\s*=\s*("|')\s*(vbscript|data):`)
)

func sanitizeReportHTML(s string) string {
	s = reScript.ReplaceAllString(s, "")
	s = reStyle.ReplaceAllString(s, "")
	s = reIframe.ReplaceAllString(s, "")
	s = reObject.ReplaceAllString(s, "")
	s = reEmbed.ReplaceAllString(s, "")
	s = reEvent.ReplaceAllString(s, "")
	s = reJSProto.ReplaceAllString(s, "$1=$2#")
	s = reDangerURI.ReplaceAllString(s, "$1=$2#")
	return s
}
