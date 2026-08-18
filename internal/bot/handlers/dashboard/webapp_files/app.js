// ============================================
// Prisma Web App - Мой профиль
// Реальные данные: грузим /api/metrics (с initData из Telegram) и рендерим.
// ============================================

let trendChart = null;

// Кэш initData: считываем один раз и переиспользуем, чтобы не зависеть от
// тайминга готовности window.Telegram.WebApp (на некоторых платформах
// глобальный объект инициализируется чуть позже, чем отрисовывается страница)
// и от того, где именно Telegram положил tgWebAppData - в query или в hash.
let cachedInitData = "";

document.addEventListener('DOMContentLoaded', () => {
    // Сообщаем Telegram, что Mini App готов, и разворачиваем на весь экран.
    try {
        if (window.Telegram && window.Telegram.WebApp) {
            window.Telegram.WebApp.ready();
            window.Telegram.WebApp.expand();
            // К моменту DOMContentLoaded telegram-web-app.js обычно уже
            // готов - захватываем initData сразу, пока он доступен.
            if (window.Telegram.WebApp.initData) {
                cachedInitData = window.Telegram.WebApp.initData;
            }
        }
    } catch (e) { /* нет TG WebApp */ }
    loadMetrics();
});

// Выводит initData изо всех возможных источников. Порядок важен:
// глобальный объект Telegram WebApp - самый надёжный, затем tgWebAppData из
// адресной строки (Telegram ВСЕГДА добавляет его при открытии Mini App).
function deriveInitData() {
    // 1) Штатный путь: глобальный объект Telegram WebApp. Доступен, когда
    //    страница открыта как Mini App (после готовности telegram-web-app.js).
    try {
        if (window.Telegram && window.Telegram.WebApp && window.Telegram.WebApp.initData) {
            return window.Telegram.WebApp.initData;
        }
    } catch (e) { /* нет TG WebApp */ }

    // 2) tgWebAppData в query-строке - основной fallback, если глобальный
    //    объект по какой-то причине ещё пуст (тайминг загрузки скрипта,
    //    частичный кэш страницы). Это и есть initData в чистом виде.
    try {
        const q = new URLSearchParams(window.location.search);
        const t1 = q.get("tgWebAppData");
        if (t1) return t1;
    } catch (e) { /* нет URLSearchParams */ }

    // 3) tgWebAppData в хэше (fragment). На ряде платформ/версий Telegram
    //    параметры запуска кладутся именно туда, а не в query.
    try {
        const h = new URLSearchParams(window.location.hash.replace(/^#/, ""));
        const t2 = h.get("tgWebAppData");
        if (t2) return t2;
    } catch (e) { /* нет */ }

    return "";
}

function getInitData() {
    // Всегда перечитываем заново (дёшево, без сети) - к моменту сабмита
    // window.Telegram.WebApp мог «проснуться» и стать доступным. Если свежий
    // перечит оказался пуст, но ранее мы уже поймали initData - отдаём кэш.
    const fresh = deriveInitData();
    if (fresh) {
        cachedInitData = fresh;
        return fresh;
    }
    if (cachedInitData) return cachedInitData;

    console.warn("[Prisma] initData недоступен: window.Telegram.WebApp пуст и tgWebAppData отсутствует в URL (query/hash).");
    return "";
}

async function loadMetrics() {
    const initData = getInitData();

    if (!initData) {
        showMessage(
            "Откройте «Мой профиль» из Telegram, чтобы увидеть ваши данные.",
            "info"
        );
        return;
    }

    // Демо-режим (?demo=1) - просим бэкенд отдать синтетические метрики,
    // чтобы можно было посмотреть графики без реальных анализов.
    const demo = new URLSearchParams(window.location.search).get("demo");
    const apiUrl = "/api/metrics?initData=" + encodeURIComponent(initData) + (demo ? "&demo=1" : "");

    try {
        const resp = await fetch(apiUrl);

        if (resp.status === 401) {
            showMessage("Не удалось проверить сессию. Откройте дашборд заново из Telegram.", "error");
            return;
        }
        if (resp.status === 403) {
            showPremiumRequired();
            return;
        }
        if (!resp.ok) {
            showMessage("Сервис временно недоступен. Попробуйте позже.", "error");
            return;
        }

        const data = await resp.json();
        render(data);
        // Карточки сохранённых отчётов (Расширенные анализы / Bioscan PRO)
        // грузим отдельным запросом - они не зависят от метрик сводки.
        loadReports(initData, demo).catch(function () {});
    } catch (e) {
        showMessage("Ошибка загрузки данных: " + e.message, "error");
    }
}

function render(data) {
    hideMessage();
    hideRegisterCard();
    hidePremiumBanner();
    hidePremiumStubs();

    document.getElementById('userName').textContent = data.userName ? data.userName : "Мой профиль";
    document.getElementById('analysisDate').textContent = data.analysisDate
        ? "Последний анализ: " + data.analysisDate
        : "Нет загруженных анализов";

    if (data.noData) {
        document.getElementById('healthIndex').textContent = "-";
        document.getElementById('energyLevel').textContent = "-";
        const recCard = document.getElementById('recommendationsCard');
        if (recCard) recCard.style.display = "none";
        showMessage("Заполните профиль - и ваш профиль оживёт.", "info");
        showRegisterCard();
        return;
    }

    // Премиум-гейт «Мой профиль» (как было раньше): без Premium
    // богатые блоки обзора скрываются, показываем только баннер с
    // предложением оформить подписку. Свои бесплатные результаты
    // (обычный анализ, базовый биоскан) видны в разделе отчётов
    // (вкладки «Анализы»/«Bioscan»), где они не гейтятся.
    if (data.premiumRequired) {
        document.getElementById('healthIndex').textContent = "-";
        document.getElementById('energyLevel').textContent = "-";
        document.getElementById('metricGroups').style.display = "none";
        document.getElementById('trendCard').style.display = "none";
        const recList = document.getElementById('recList');
        if (recList) recList.innerHTML = "";
        const recCard = document.getElementById('recommendationsCard');
        if (recCard) recCard.style.display = "none";
        document.getElementById('lastUpdate').textContent = "Обновлено: " + new Date().toLocaleString('ru-RU');
        showPremiumBanner();
        // Показываем заглушки-тизеры: пользователь видит, какие карточки
        // с анализами и прогрессом появятся после оформления Premium.
        showPremiumStubs();
        return;
    }

    // Индексы
    const hi = document.getElementById('healthIndex');
    hi.textContent = data.healthIndex;
    hi.style.color = data.healthIndex >= 80 ? '#1FA6A8' : data.healthIndex >= 60 ? '#E8744A' : '#D32F2F';
    document.getElementById('energyLevel').textContent = data.energyLevel || "-";

    // Адаптивные блоки показателей из реальных анализов/биосканов.
    renderMetricGroups(data.groups);

    // Рекомендации
    const recCard = document.getElementById('recommendationsCard');
    if (recCard) recCard.style.display = "block";
    const recList = document.getElementById('recList');
    recList.innerHTML = "";
    (data.recommendations || []).forEach(r => {
        const li = document.createElement('li');
        li.textContent = r;
        recList.appendChild(li);
    });

    // Динамика: текущее значение индекса на шкале 0-100 всегда видно;
    // линейный график (изменение во времени) рисуем только при 2+ замерах
    // - иначе точка одна и «график» не несёт информации.
    const trend = data.trend || { labels: [], values: [] };
    if (trend.values && trend.values.length >= 1) {
        document.getElementById('trendCard').style.display = 'block';
        const latest = trend.values[trend.values.length - 1];
        const color = latest >= 80 ? '#1FA6A8' : latest >= 60 ? '#E8744A' : '#D32F2F';
        const tv = document.getElementById('trendValue');
        tv.textContent = latest;
        tv.style.color = color;
        const td = document.getElementById('trendDate');
        if (td) td.textContent = (trend.labels && trend.labels.length) ? trend.labels[trend.labels.length - 1] : '';
        const fill = document.getElementById('trendScaleFill');
        if (fill) {
            fill.style.width = Math.max(0, Math.min(100, latest)) + '%';
            fill.style.background = color;
        }
        const hint = document.getElementById('trendHint');
        const chartWrap = document.getElementById('trendChartWrap');
        const placeholder = document.getElementById('trendPlaceholder');
        const n = trend.values.length;
        if (n >= 2) {
            // Два и более замеров - рисуем полную линию динамики и
            // подписываем их число, чтобы было понятно, на каком
            // объёме данных построена динамика.
            if (chartWrap) { chartWrap.style.display = 'block'; drawTrend(trend.labels, trend.values); }
            if (placeholder) placeholder.style.display = 'none';
            if (hint) hint.textContent = 'Динамика по ' + n + ' замерам. Повторяйте анализ раз в 2-4 недели, чтобы отслеживать изменения.';
        } else {
            // Один замер - линию динамики построить нельзя. В «Обзоре»
            // плашку не показываем (она перенесена в раздел «Анализы»);
            // оставляем только текущий индекс. Скрываем график и
            // уничтожаем ранее отрисованный (чтобы в скрытом канвасе
            // не висела старая точка/линия).
            if (trendChart) { try { trendChart.destroy(); } catch (e) {} trendChart = null; }
            if (chartWrap) chartWrap.style.display = 'none';
            if (placeholder) placeholder.style.display = 'none';
            if (hint) hint.textContent = '';
        }
    } else {
        document.getElementById('trendCard').style.display = 'none';
    }

    document.getElementById('lastUpdate').textContent = "Обновлено: " + new Date().toLocaleString('ru-RU');
}

// renderMetricGroups - заполняет контейнер #metricGroups адаптивными
// блоками показателей из реальных отчётов пользователя: категории последнего
// анализа (кровь/биохимия/гормоны/...) + блок тела Bioscan PRO. Каждый блок
// - отдельная карточка; строки подсвечиваются по статусу (норма/внимание/
// критично) через классы .ind-status-*. Если групп нет - контейнер скрыт.
function renderMetricGroups(groups) {
    const wrap = document.getElementById('metricGroups');
    if (!wrap) return;
    wrap.innerHTML = "";
    if (!groups || groups.length === 0) {
        wrap.style.display = "none";
        return;
    }
    wrap.style.display = "block";

    groups.forEach(function (g) {
        if (!g || !g.items || g.items.length === 0) return;
        const card = document.createElement('section');
        card.className = 'card metric-group';

        const head = document.createElement('h2');
        head.className = 'mg-head';
        head.textContent = (g.icon ? g.icon + ' ' : '') + (g.title || '');
        card.appendChild(head);

        const gridEl = document.createElement('div');
        gridEl.className = 'mg-grid';
        g.items.forEach(function (it) {
            const row = document.createElement('div');
            row.className = 'mg-item';
            const status = (it.status || 'normal');
            const known = ['normal', 'warning', 'critical', 'good'];
            const statusClass = 'ind-status-' + (known.indexOf(status) >= 0 ? status : 'normal');
            row.innerHTML =
                '<span class="mg-name">' + escapeHtml(it.name) + '</span>' +
                '<span class="mg-value ' + statusClass + '">' + escapeHtml(it.value || '-') + '</span>';
            gridEl.appendChild(row);
        });
        card.appendChild(gridEl);
        wrap.appendChild(card);
    });
}

function drawTrend(labels, values) {
    if (typeof Chart === 'undefined') {
        // Библиотека графиков недоступна (CDN заблокирован/медленный) -
        // не падаем, остальная часть дашборда продолжает работать.
        return;
    }
    const ctx = document.getElementById('chartTrend').getContext('2d');
    const grad = ctx.createLinearGradient(0, 0, 0, 180);
    grad.addColorStop(0, 'rgba(31, 166, 168, 0.30)');
    grad.addColorStop(1, 'rgba(31, 166, 168, 0.02)');

    if (trendChart) trendChart.destroy();
    trendChart = new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: 'Индекс здоровья',
                data: values,
                borderColor: '#1FA6A8',
                backgroundColor: grad,
                borderWidth: 3,
                fill: true,
                tension: 0.4,
                pointRadius: 4,
                pointHoverRadius: 6
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: { min: 0, max: 100, grid: { color: 'rgba(255,255,255,0.06)' } },
                x: { grid: { display: false } }
            },
            plugins: { legend: { display: false } }
        }
    });
}

function showRegisterCard() {
    document.getElementById('registerCard').style.display = 'block';
    document.getElementById('regError').style.display = 'none';
    // Обработчик кнопки навешивается ОДИН раз при загрузке скрипта
    // (см. bindSaveButton внизу), чтобы кнопка была рабочей даже если
    // render() по какой-то причине не отработал до конца.
}

function hideRegisterCard() {
    document.getElementById('registerCard').style.display = 'none';
}

async function submitProfile() {
    const initData = getInitData();
    const errEl = document.getElementById('regError');
    const btn = document.getElementById('regSubmit');
    if (errEl) errEl.style.display = 'none';

    if (!initData) {
        if (errEl) {
            errEl.textContent = "Не удалось получить данные сессии Telegram. Убедитесь, что открыли «Мой профиль» через кнопку бота внутри Telegram (а не в обычном браузере), и попробуйте ещё раз.";
            errEl.style.display = 'block';
        }
        return;
    }

    const name = document.getElementById('regName').value.trim();
    const age = parseInt(document.getElementById('regAge').value, 10);
    const gender = document.getElementById('regGender').value;
    const height = parseInt(document.getElementById('regHeight').value, 10) || 0;
    const weight = parseInt(document.getElementById('regWeight').value, 10) || 0;
    const goal = document.getElementById('regGoal').value.trim();

    if (name.length < 2) {
        if (errEl) {
            errEl.textContent = "Укажите имя (минимум 2 символа).";
            errEl.style.display = 'block';
        }
        return;
    }
    if (!Number.isFinite(age) || age < 5 || age > 90) {
        if (errEl) {
            errEl.textContent = "Укажите возраст от 5 до 90.";
            errEl.style.display = 'block';
        }
        return;
    }

    const payload = { name, age, gender, height, weight, goal };

    // Блокируем повторные нажатия и показываем статус сохранения.
    if (btn) {
        btn.disabled = true;
        btn.textContent = "Сохранение…";
    }

    try {
        const resp = await fetch("/api/profile?initData=" + encodeURIComponent(initData), {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(payload)
        });
        if (resp.status === 401) {
            showMessage("Не удалось проверить сессию. Откройте дашборд заново из Telegram.", "error");
            return;
        }
        if (!resp.ok) {
            let msg = "Не удалось сохранить профиль. Попробуйте ещё раз.";
            try {
                const j = await resp.json();
                if (j && j.error) msg = "Ошибка: " + j.error;
            } catch (_) { /* тело не JSON - оставляем дефолт */ }
            if (errEl) {
                errEl.textContent = msg;
                errEl.style.display = 'block';
            }
            return;
        }
        hideRegisterCard();
        loadMetrics();
    } catch (e) {
        if (errEl) {
            errEl.textContent = "Ошибка сети: " + e.message;
            errEl.style.display = 'block';
        }
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.textContent = "Сохранить профиль";
        }
    }
}

function showMessage(text, kind) {
    const card = document.getElementById('messageCard');
    const p = document.getElementById('messageText');
    p.textContent = text;
    card.style.display = 'block';
}

function showPremiumRequired() {
    const card = document.getElementById('messageCard');
    const p = document.getElementById('messageText');
    // Просто плашка: сообщаем, что нужна Premium, и как её получить.
    // Никаких кнопок/переходов - пользователь разберётся самостоятельно
    // (в боте есть кнопка меню «💎 Premium»). Если Premium активна,
    // эта ветка не вызывается (см. data.premiumRequired в loadMetrics/render).
    p.textContent = "💎 Для просмотра «Моего профиля» нужна Premium-подписка. Оформите её в боте - нажмите кнопку «💎 Premium» в меню.";
    card.style.display = 'block';
}

function hideMessage() {
    document.getElementById('messageCard').style.display = 'none';
}

// Показывает НЕблокирующую подсказку про Premium. Реальные данные
// пользователя уже отрисованы выше - баннер лишь предлагает расширить
// возможности (расширенный анализ-досье, Bioscan PRO) по подписке.
function showPremiumBanner() {
    const el = document.getElementById('premiumBanner');
    if (el) el.style.display = 'block';
}

function hidePremiumBanner() {
    const el = document.getElementById('premiumBanner');
    if (el) el.style.display = 'none';
}

// Показывает заглушки-тизеры на вкладке «Обзор»: пользователь без Premium
// видит, какие карточки с анализами и прогрессом появятся после подписки.
// Контейнер использует класс .metric-groups (display:flex, колонка).
function showPremiumStubs() {
    const el = document.getElementById('premiumStubs');
    if (el) el.style.display = 'flex';
}

function hidePremiumStubs() {
    const el = document.getElementById('premiumStubs');
    if (el) el.style.display = 'none';
}

// ---------------------------------------------------------------
// Вкладки «Мой профиль» (Обзор / Анализы / Bioscan PRO).
// Chart.js не рисует корректно в скрытых контейнерах (нулевой размер
// холста), поэтому при показе вкладки отчётов перерисовываем её графики.
// ---------------------------------------------------------------

let activeTab = "overview";

function showTab(tab) {
    activeTab = tab;
    const panels = document.querySelectorAll(".tab-panel");
    panels.forEach(function (p) { p.classList.remove("active"); });
    const btns = document.querySelectorAll(".tab-btn");
    btns.forEach(function (b) { b.classList.remove("active"); });
    const panel = document.getElementById("tab-" + tab);
    if (panel) panel.classList.add("active");
    const btn = document.querySelector('.tab-btn[data-tab="' + tab + '"]');
    if (btn) btn.classList.add("active");

    if (tab === "analysis" && window.__reportsData) {
        renderReportGroup("analysis", window.__reportsData.analysis, window.__reportsData.premiumRequired);
    } else if (tab === "bioscan" && window.__reportsData) {
        renderReportGroup("bioscan", window.__reportsData.bioscan, window.__reportsData.premiumRequired);
    } else if (tab === "monitoring") {
        // Мониторинг теперь встроен в «Мой профиль» как отдельная вкладка
        // (iframe). Грузим лениво при первом открытии вкладки.
        loadMonitoringFrame();
    }
}

// loadMonitoringFrame - лениво грузит вкладку «Мониторинг» как iframe внутрь
// единого «Моего профиля». Мониторинг переехал сюда из отдельного Mini App:
// его статика и API (/monitoring, /api/monitoring/...) уже обслуживаются тем
// же бэкендом, поэтому переиспользуем их как вложенный фрейм. initData и
// демо-флаг пробрасываем в URL, чтобы внутри фрейма работала авторизация
// API и (в демо-режиме) синтетические проекты. Грузим один раз - при первом
// открытии вкладки (к этому моменту панель уже видима, графики рисуются
// корректно, без «нулевого» размера холста).
let monitoringFrameLoaded = false;
function loadMonitoringFrame() {
    const frame = document.getElementById("monitoringFrame");
    if (!frame || monitoringFrameLoaded) return;
    const demo = new URLSearchParams(window.location.search).get("demo");
    let src = "/monitoring?initData=" + encodeURIComponent(getInitData());
    if (demo) src += "&demo=1";
    frame.src = src;
    monitoringFrameLoaded = true;
}

// openReportPDF - открывает сохранённый отчёт (PDF/HTML) прямо из Сводки.
// Бэкенд /api/reports/file проверяет подлинность сессии и владельца отчёта
// и отдаёт HTML (или PDF при недоступности HTML-фоллбэка).
//
// Стратегия открытия (отдаём приоритет встроенному просмотру):
//  ОСНОВНОЙ путь - встроенный просмотрщик-оверлей (iframe) прямо внутри
//  Mini App. URL - тот же домен, что и у дашборда (?view=inline → бэкенд
//  отдаёт HTML), поэтому Telegram НЕ показывает окно «посетить сайт» (оно
//  появляется только при внешней навигации / window.open). Отчёт рендерится
//  прямо в приложении, закрывается кнопкой «✕ Закрыть».
//  ЗАПАСНОЙ путь - window.open(url, '_blank') в отдельной in-app вкладке,
//  используется только если элемент оверлея по какой-то причине недоступен.
// НЕ используем window.location.href: навигация текущего Mini App на
// PDF/HTML «белит» приложение или скачивает файл в никуда.
function openReportPDF(kind, id) {
    let base;
    if (window.__demo) {
        // Демо-режим: отчёт синтетический, не привязан к сессии/БД -
        // открываем через специальный demo=1 эндпоинт (без initData).
        base = "/api/reports/file?demo=1&type=" + encodeURIComponent(kind) +
            "&id=" + encodeURIComponent(id);
    } else {
        const initData = getInitData();
        if (!initData) {
            showMessage("Откройте «Мой профиль» через бота в Telegram, чтобы открыть отчёт.", "error");
            return;
        }
        base = "/api/reports/file?initData=" + encodeURIComponent(initData) +
            "&type=" + encodeURIComponent(kind) + "&id=" + encodeURIComponent(id);
    }

    // ОСНОВНОЙ путь: встроенный просмотрщик (iframe) внутри Mini App.
    // ?view=inline → бэкенд отдаёт HTML (без PDF-конвертации), надёжно
    // рендерится в WebView и не триггерит окно «посетить сайт» Telegram.
    if (document.getElementById("reportViewer")) {
        openReportInline(base + "&view=inline");
        return;
    }

    // ЗАПАСНОЙ путь: отдельная вкладка (in-app браузер Telegram).
    try {
        window.open(base, "_blank");
    } catch (e) { /* ничего не открылось */ }
}

// openReportInline - показывает оверлей с iframe, грузящим отчёт.
function openReportInline(url) {
    const viewer = document.getElementById("reportViewer");
    const frame = document.getElementById("reportViewerFrame");
    if (!viewer || !frame) {
        // На крайний случай - навигация текущей вкладки.
        window.location.href = url;
        return;
    }
    frame.src = url;
    viewer.style.display = "flex";
    viewer.scrollTop = 0;
}

// confirmDelete - запрашивает подтверждение удаления. Внутри Telegram WebApp
// использует нативный showConfirm (единый стиль с приложением); в обычном
// браузере - стандартный window.confirm. Возвращает Promise<bool>.
function confirmDelete(text) {
    return new Promise(function (resolve) {
        try {
            if (window.Telegram && window.Telegram.WebApp && window.Telegram.WebApp.showConfirm) {
                window.Telegram.WebApp.showConfirm(text, function (ok) { resolve(!!ok); });
                return;
            }
        } catch (e) { /* игнор - упадём на window.confirm */ }
        resolve(window.confirm(text));
    });
}

// deleteEntry - удаляет отчёт/анализ по ID из «Мой профиль».
// Сначала спрашивает подтверждение, затем шлёт DELETE на бэкенд и, при
// успехе, перезагружает метрики и карточки отчётов - чтобы архив,
// графики и блок «Динамика» (в т.ч. подсказка «Пока один замер»)
// обновились без перезапуска Mini App.
async function deleteEntry(kind, id) {
    if (!id || id <= 0) return;
    const ok = await confirmDelete("Удалить этот отчёт? Данные нельзя будет восстановить.");
    if (!ok) return;

    const initData = getInitData();
    if (!initData) {
        showMessage("Не удалось получить сессию Telegram. Откройте «Мой профиль» через кнопку бота внутри Telegram, чтобы удалить отчёт.", "error");
        return;
    }

    try {
        const resp = await fetch("/api/reports/delete?id=" + encodeURIComponent(id) + "&initData=" + encodeURIComponent(initData), {
            method: "DELETE"
        });
        if (resp.status === 401) {
            showMessage("Не удалось проверить сессию. Откройте дашборд заново из Telegram.", "error");
            return;
        }
        if (!resp.ok) {
            showMessage("Не удалось удалить отчёт. Попробуйте ещё раз.", "error");
            return;
        }
        // Данные удалены - перезагружаем сводку и карточки отчётов.
        loadMetrics();
    } catch (e) {
        showMessage("Ошибка сети: " + e.message, "error");
    }
}

// bindReportViewer - навешивает закрытие оверлея (один раз при загрузке).
(function bindReportViewer() {
    const viewer = document.getElementById("reportViewer");
    const frame = document.getElementById("reportViewerFrame");
    const closeBtn = document.getElementById("reportViewerClose");
    if (closeBtn) {
        closeBtn.addEventListener("click", function () {
            if (viewer) viewer.style.display = "none";
            if (frame) frame.src = "about:blank";
        });
    }
    if (viewer) {
        // Закрытие по клику на затемнённую область вокруг содержимого.
        viewer.addEventListener("click", function (e) {
            if (e.target === viewer) {
                viewer.style.display = "none";
                if (frame) frame.src = "about:blank";
            }
        });
    }
})();

// ---------------------------------------------------------------
// Гарантированная привязка кнопки «Сохранить профиль».
// app.js грузится с defer (в конце <body>), поэтому DOM уже готов.
// Навешиваем обработчик ОДИН раз здесь, а не в render()/showRegisterCard,
// чтобы кнопка была рабочей даже если render() по какой-то причине не
// отработал (ошибка сети при загрузке метрик, частично закэшированный
// старый app.js). Иначе форма видна, но «Сохранить» не срабатывает.
(function bindSaveButton() {
    const btn = document.getElementById('regSubmit');
    if (btn) btn.addEventListener('click', submitProfile);
})();

// ---------------------------------------------------------------
// Карточки сохранённых отчётов (Расширенные анализы / Bioscan PRO)
// Данные приходят из /api/reports: последний отчёт + предыдущий + дельта.
// ---------------------------------------------------------------

async function loadReports(initData, demo) {
    // Запоминаем демо-режим, чтобы openReportPDF знал, как открывать отчёт.
    window.__demo = demo;
    const url = "/api/reports?initData=" + encodeURIComponent(initData) + (demo ? "&demo=1" : "");
    try {
        const resp = await fetch(url);
        if (!resp.ok) return;
        const data = await resp.json();
        renderReports(data);
        window.__reportsData = data;
        // Если сейчас открыта вкладка отчётов, перерисуем её графики: они
        // могли отрисоваться в скрытом состоянии с нулевым размером.
        if (activeTab === "analysis") {
            renderReportGroup("analysis", data.analysis, data.premiumRequired);
        } else if (activeTab === "bioscan") {
            renderReportGroup("bioscan", data.bioscan, data.premiumRequired);
        }
    } catch (e) { /* отдельный запрос - не ломаем сводку при ошибке */ }
}

function renderReports(data) {
    if (!data) return;
    renderReportGroup("analysis", data.analysis, data.premiumRequired);
    renderReportGroup("bioscan", data.bioscan, data.premiumRequired);
}

function cap(s) { return s.charAt(0).toUpperCase() + s.slice(1); }

function escapeHtml(s) {
    return String(s == null ? "" : s).replace(/[&<>"']/g, function (m) {
        return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[m];
    });
}

// indicatorRowHTML - строка показателя для карточки отчёта. Рисует
// вертикальный индикатор (колонку) с тремя зонами: зелёная = норма
// (низ), жёлтая = внимание (середина), красная = критично (верх).
// Маркер (--c + --p) ставится на шкалу по степени отклонения от нормы
// и окрашивается в цвет зоны. Без нормы (нет референса) шкала не
// рисуется - показываем только цифру и текстовый статус.
function indicatorRowHTML(ind) {
    const name = escapeHtml(ind.name || "");
    const value = escapeHtml(ind.value || "");
    const status = ind.status || "normal";
    const statusClass = "ind-status-" + status;
    const color = statusColor(status);
    const sev = indicatorSeverity(ind);

    let viz;
    if (sev === null) {
        // Нет референсных значений - колонка без зон (норма неизвестна).
        viz = '<div class="ind-viz ind-viz-empty" title="Норма не указана"></div>';
    } else {
        // --p: 0 = низ (норма), 1 = верх (критично); --c: цвет зоны.
        viz = '<div class="ind-viz" style="--c:' + color + '; --p:' + sev + ';" title="Норма: ' + escapeHtml(ind.normal || '') + '">' +
                '<div class="ind-marker"></div>' +
              '</div>';
    }

    const statusText = statusLabelRU(status) + (ind.normal ? ' · Норма: ' + escapeHtml(ind.normal) : '');
    const body =
        '<div class="ind-body">' +
            '<div class="ind-num ' + statusClass + '">' + value + '</div>' +
            '<div class="ind-name">' + name + '</div>' +
            '<div class="ind-status ' + statusClass + '">' + statusText + '</div>' +
        '</div>';

    return '<div class="ind-row">' + viz + body + '</div>';
}

// indicatorSeverity - насколько значение отклонено от нормы, в диапазоне
// 0..1 (0 = в центре нормы, 1 = далеко за пределами). Маркер вертикального
// индикатора берётся из этой величины: низ - норма (зелёная зона),
// верх - критично (красная).
function indicatorSeverity(ind) {
    const v = Number(ind.num), lo = Number(ind.refMin), hi = Number(ind.refMax);
    if (!(v > 0) || !(hi > lo)) return null;
    const span = hi - lo;
    let sev;
    if (v >= lo && v <= hi) {
        const center = (lo + hi) / 2;
        sev = 0.33 * (Math.abs(v - center) / (span / 2));
    } else if (v > hi) {
        const over = (v - hi) / span;
        sev = 0.33 + 0.67 * Math.min(over / 2, 1);
    } else {
        const under = (lo - v) / span;
        sev = 0.33 + 0.67 * Math.min(under / 2, 1);
    }
    return Math.max(0, Math.min(1, sev));
}

function statusColor(s) {
    if (s === "critical") return "#E5484D";
    if (s === "warning") return "#E8744A";
    return "#4F8A6D";
}

function statusLabelRU(s) {
    if (s === "critical") return "Критично";
    if (s === "warning") return "Внимание";
    if (s === "normal" || s === "good") return "Норма";
    return "-";
}

function renderReportGroup(kind, group, premiumRequired) {
    const card = document.getElementById(kind + "ReportCard");
    if (!card) return;
    const latest = group && group.latest;
    const ph = document.getElementById(kind + "Placeholder");
    if (!group || !latest || !latest.available) {
        card.style.display = "none";
        if (ph) ph.style.display = "block";
        return;
    }

    // Премиум-гейт (как было раньше): без Premium прячем «богатый»
    // контент (scores/zones/indicators/сравнение), который уже очищен
    // на бэкенде для не-Premium. Бесплатные результаты (обычный анализ,
    // базовый биоскан) не имеют этих полей, поэтому их карточки и архив
    // остаются видимыми - это собственные данные пользователя.
    const premiumBlocked = premiumRequired && latest.rich;
    if (premiumBlocked) {
        // Только премиум-отчёт и нет бесплатных в архиве - прячем
        // карточку и показываем заглушку.
        const hasFree = (group.reports || []).some(function (r) {
            return r && r.available && !r.rich;
        });
        if (!hasFree) {
            card.style.display = "none";
            if (ph) ph.style.display = "block";
            return;
        }
    }

    card.style.display = "block";
    if (ph) ph.style.display = "none";

    // Видимые (доступные) отчёты с учётом премиум-гейта: премиум-отчёты
    // (rich) без подписки прячем, оставляем бесплатные. Используется ниже
    // для решения «показывать radar-график/плашку» и построения архива.
    const visible = (group.reports || []).filter(function (r) {
        return r && r.available && !(premiumRequired && r.rich);
    });

    document.getElementById(kind + "ReportTitle").textContent =
        latest.title || (kind === "bioscan" ? "Bioscan PRO" : "Расширенный анализ");
    document.getElementById(kind + "ReportDate").textContent = "от " + (latest.date || "-");

    // Заголовок раздела отражает РЕАЛЬНЫЙ тип отчёта: расширенный
    // анализ-досье (rich=true) vs обычный анализ, Bioscan PRO vs базовый
    // Bioscan. Без этого обычный анализ (напр. иммуноглобулин) ошибочно
    // подписывался как «Расширенные анализы».
    const headerEl = document.getElementById(kind + "ReportHeader");
    if (headerEl) {
        if (kind === "bioscan") {
            headerEl.textContent = latest.rich ? "✨ Bioscan PRO" : "✨ Базовый Bioscan";
        } else {
            headerEl.textContent = latest.rich ? "📊 Расширенный анализ" : "📊 Анализ";
        }
    }

    // Счёт (индекс/Body Score). Показываем только если он есть - иначе
    // рядом с названием висит бессмысленная «0».
    const scoreEl = document.getElementById(kind + "ReportScore");
    if (latest.mainScore && latest.mainScore > 0) {
        scoreEl.innerHTML = latest.mainScore + (latest.scoreLabel ? "<small>" + latest.scoreLabel + "</small>" : "");
        scoreEl.style.display = "block";
    } else {
        scoreEl.style.display = "none";
    }

    // Кликабельна ТОЛЬКО «шапка» отчёта (заголовок + дата + счёт), а не
    // весь раздел карточки: графики, индикаторы, сравнение и архив -
    // отдельные элементы (архив уже кликабелен по строкам). Клик шапки
    // открывает HTML-отчёт напрямую; формат (HTML/PDF) выбирается внутри
    // самого отчёта.
    const headEl = document.getElementById(kind + "ReportHead");
    if (headEl) {
        if (latest.id && latest.id > 0) {
            headEl.style.cursor = "pointer";
            headEl.onclick = function () { openReportPDF(kind, latest.id); };
        } else {
            headEl.style.cursor = "default";
            headEl.onclick = null;
        }
    }

    // Radar-график по набору оценок. Показываем только при 2+ отчётах:
    // при одном анализе под плашкой «Пока один отчёт» один radar со срезом
    // показателей лишь визуально мусорит, сравнивать и строить динамику
    // не с чем. При скрытии уничтожаем график, чтобы в скрытом канвасе
    // не висел старый рисунок.
    const wrap = document.getElementById(kind + "ChartWrap");
    const scores = premiumBlocked ? {} : (latest.scores || {});
    const labels = Object.keys(scores);
    if (wrap) {
        if (labels.length > 0 && visible.length >= 2) {
            wrap.style.display = "block";
            drawReportRadar("chart" + cap(kind), labels, labels.map(function (k) { return scores[k]; }));
        } else {
            wrap.style.display = "none";
            if (window.__reportCharts && window.__reportCharts["chart" + cap(kind)]) {
                try { window.__reportCharts["chart" + cap(kind)].destroy(); } catch (e) {}
                window.__reportCharts["chart" + cap(kind)] = null;
            }
        }
    }

    // Индикаторы (список) - для анализа. Каждый показатель рисуем
    // вертикальным индикатором с зонами (зелёная=норма, жёлтая=внимание,
    // красная=критично) и меткой значения. Если референс неизвестен
    // (нет нормы) - шкалу не рисуем, показываем цифру и текстовый статус.
    const indEl = document.getElementById(kind + "Indicators");
    if (indEl) {
        indEl.innerHTML = "";
        const inds = premiumBlocked ? [] : (latest.indicators || []);
        inds.slice(0, 8).forEach(function (ind) {
            const row = document.createElement("div");
            row.className = "ind-row";
            row.innerHTML = indicatorRowHTML(ind);
            indEl.appendChild(row);
        });
    }

    // Зоны (круговые диаграммы) - для Bioscan PRO.
    const zonesEl = document.getElementById(kind + "Zones");
    if (zonesEl) {
        zonesEl.innerHTML = "";
        const zones = premiumBlocked ? [] : (latest.zones || []);
        zones.forEach(function (z) {
            const cell = document.createElement("div");
            cell.className = "zdonut-cell";
            const status = z.status === "critical" ? "critical" : (z.status === "warning" ? "warning" : "good");
            cell.innerHTML = '<div class="zdonut ' + status + '" style="--p:' + (z.score || 0) + '">' + (z.score || 0) + '</div>' +
                '<div class="zname">' + escapeHtml(z.name) + '</div>';
            zonesEl.appendChild(cell);
        });
    }


    // Визуальный архив ВСЕХ сохранённых отчётов. Без Premium прячем
    // премиум-отчёты (rich), оставляем бесплатные.
    renderArchive(kind, group.reports, group.latest.date, premiumRequired);

    // Подсказка: для не-Premium - апселл, иначе - guidance при малом
    // числе отчётов (один отчёт = ещё нечего сравнивать, график бессмыслен).
    // count «видимых» отчётов (visible) вычислен выше.

    // Крупная плашка-подсказка на главном месте раздела «Анализы»:
    // показываем, когда загружен только один отчёт (сравнить и
    // построить динамику не с чем). В остальных случаях прячем.
    const phEl = document.getElementById(kind + "TrendPlaceholder");
    const phTextEl = document.getElementById(kind + "TrendPlaceholderText");
    const phSubEl = document.getElementById(kind + "TrendPlaceholderSub");
    if (phEl) {
        if (kind === "analysis" && !premiumRequired && visible.length <= 1) {
            phEl.style.display = "flex";
            if (phTextEl) {
                phTextEl.textContent = "Пока один отчёт" +
                    (latest.date ? " (" + latest.date + ")" : "") +
                    ". Загрузите следующий анализ, чтобы увидеть динамику.";
            }
            if (phSubEl) phSubEl.textContent = "График появится, когда накопится 2 и более замеров.";
        } else {
            phEl.style.display = "none";
        }
    }

    const hint = document.getElementById(kind + "Hint");
    if (hint) {
        if (premiumRequired) {
            hint.textContent = "Расширенный анализ-досье и Bioscan PRO доступны на Premium. Оформите подписку в боте (кнопка 💎 Premium).";
        } else if (kind === "analysis") {
            // Для «Анализов» крупная плашка выше заменяет мелкую подсказку.
            hint.textContent = "";
        } else {
            hint.textContent = visible.length <= 1
                ? "Пока один отчёт. Загрузите следующий анализ или биоскан, чтобы увидеть динамику."
                : "";
        }
    }
}

function drawReportRadar(canvasId, labels, values) {
    const canvas = document.getElementById(canvasId);
    if (!canvas || typeof Chart === 'undefined') return;
    const ctx = canvas.getContext("2d");
    if (window.__reportCharts && window.__reportCharts[canvasId]) {
        window.__reportCharts[canvasId].destroy();
    }
    const chart = new Chart(ctx, {
        type: "radar",
        data: {
            labels: labels,
            datasets: [{
                label: "Оценка",
                data: values,
                borderColor: "#1FA6A8",
                backgroundColor: "rgba(31,166,168,0.25)",
                pointBackgroundColor: "#1FA6A8",
                borderWidth: 2
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                r: {
                    min: 0, max: 100,
                    ticks: { display: false, stepSize: 20 },
                    grid: { color: "rgba(255,255,255,0.08)" },
                    angleLines: { color: "rgba(255,255,255,0.08)" },
                    pointLabels: { color: "#8b9aa7", font: { size: 10 } }
                }
            },
            plugins: { legend: { display: false } }
        }
    });
    window.__reportCharts = window.__reportCharts || {};
    window.__reportCharts[canvasId] = chart;
}


// renderArchive - визуальный архив ВСЕХ сохранённых отчётов пользователя
// (история прогресса). Каждый отчёт - строка с мини-донатом индекса,
// датой и подсветкой текущего. Позволяет видеть, как менялись показатели
// от замера к замеру, прямо в «Мой профиль».
function renderArchive(kind, reports, latestDate, premiumRequired) {
    const wrap = document.getElementById(kind + "Archive");
    const list = document.getElementById(kind + "ArchiveList");
    if (!wrap || !list) return;
    if (!reports || reports.length === 0) { wrap.style.display = "none"; return; }
    wrap.style.display = "block";
    list.innerHTML = "";

    reports.forEach(function (r) {
        if (!r || !r.available) return;
        // Без Premium прячем премиум-отчёты (rich), оставляем бесплатные.
        if (premiumRequired && r.rich) return;
        const score = r.mainScore || 0;
        const color = score >= 80 ? "var(--ok)" : (score >= 60 ? "var(--warn)" : "#E5484D");
        const isLatest = r.date === latestDate;
        const div = document.createElement("div");
        div.className = "archive-item" + (isLatest ? " latest" : "");
        // Донат скрываем, если счёта нет (бесплатный отчёт без индекса),
        // чтобы не висела бессмысленная «0».
        const donut = score > 0
            ? '<div class="archive-donut" style="background: conic-gradient(' + color + ' ' + score + '%, rgba(255,255,255,0.06) 0);">' + score + '</div>'
            : '<div class="archive-donut archive-donut-empty"></div>';
        // Кнопка удаления отчёта. В демо-режиме отчёты синтетические и не
        // привязаны к БД - удалять нечего, поэтому кнопку не показываем.
        const delBtn = (window.__demo || !(r.id && r.id > 0))
            ? ''
            : '<button class="archive-del" type="button" title="Удалить отчёт" aria-label="Удалить отчёт">🗑</button>';
        div.innerHTML =
            donut +
            '<div class="archive-info">' +
                '<div class="archive-name">' + escapeHtml(r.title || (kind === "bioscan" ? "Bioscan PRO" : "Расширенный анализ")) +
                (isLatest ? ' <span class="archive-badge">текущий</span>' : '') + '</div>' +
                '<div class="archive-date">' + escapeHtml(r.date || "") + '</div>' +
            '</div>' +
            delBtn +
            (isLatest ? '' : '<div class="archive-go">›</div>');
        if (r.id && r.id > 0) {
            div.style.cursor = "pointer";
            div.onclick = function (e) {
                // Не триггерим клик по всей карточке - открываем именно
                // этот отчёт из архива.
                e.stopPropagation();
                openReportPDF(kind, r.id);
            };
            // Кнопка удаления: гасим всплытие, чтобы не открыть отчёт,
            // и запускаем удаление с подтверждением.
            const del = div.querySelector(".archive-del");
            if (del) {
                del.onclick = function (e) {
                    e.stopPropagation();
                    deleteEntry(kind, r.id);
                };
            }
        }
        list.appendChild(div);
    });
}
