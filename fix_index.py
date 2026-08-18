# -*- coding: utf-8 -*-
import io, sys

path = "internal/bot/handlers/dashboard/webapp_files/index.html"
with io.open(path, "r", encoding="utf-8") as f:
    text = f.read()

orig = text

text = text.replace("style.v35.css", "style.v36.css")
assert "style.v36.css" in text, "css version not replaced"
text = text.replace("app.v35.js", "app.v36.js")
assert "app.v36.js" in text, "js version not replaced"

text = text.replace(
    "onclick=\"showTab('overview')\">📋 Обзор</button>",
    "onclick=\"showTab('overview')\">📊 Обзор</button>",
)
assert "📊 Обзор</button>" in text, "overview icon not replaced"

text = text.replace(
    "onclick=\"showTab('bioscan')\">✨ Bioscan</button>",
    "onclick=\"showTab('bioscan')\">📸 Bioscan</button>\n"
    "                <button class=\"tab-btn\" type=\"button\" data-tab=\"monitoring\" onclick=\"showTab('monitoring')\">💉 Мониторинг</button>",
)
assert "💉 Мониторинг</button>" in text, "monitoring tab button not inserted"

marker = "            <!-- Состояния (всегда видимы, вне вкладок) -->"
assert marker in text, "states marker not found"
tab_block = (
    "            <!-- Вкладка «Мониторинг»: встроенное веб-приложение мониторинга "
    "внутри единого «Моего профиля». Грузится лениво при первом открытии "
    "вкладки (см. loadMonitoringFrame в app.js). -->\n"
    "            <section class=\"tab-panel\" id=\"tab-monitoring\">\n"
    "                <iframe id=\"monitoringFrame\" class=\"monitoring-frame\" title=\"Мониторинг\" frameborder=\"0\"></iframe>\n"
    "            </section>\n\n"
)
text = text.replace(marker, tab_block + marker)
assert "id=\"tab-monitoring\"" in text, "monitoring tab panel not inserted"

if text == orig:
    print("NO_CHANGES")
    sys.exit(1)

with io.open(path, "w", encoding="utf-8") as f:
    f.write(text)
print("OK_WRITTEN bytes=", len(text.encode("utf-8")))
