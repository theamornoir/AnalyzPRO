// ============================================================
// Prisma — Мониторинг (Telegram Mini App)
// SPA: список проектов, создание, привязка записей истории, графики.
// ============================================================

const tg = window.Telegram && window.Telegram.WebApp;

// initData подписывается Telegram при открытии Mini App. Читаем его «лениво»
// (в момент запроса), чтобы гарантированно получить актуальное значение,
// даже если скрипт Telegram инициализируется чуть позже загрузки app.js.
function getInitData() {
    if (tg && tg.initData) return tg.initData;
    return new URLSearchParams(location.search).get('initData') || '';
}

// Типы мониторинга (синхронизированы с backend ProjectTypes).
const PROJECT_TYPES = [
    { value: 'course',   label: '💉 Курс препаратов' },
    { value: 'diabetes', label: '🩸 Диабет' },
    { value: 'weight',   label: '⚖️ Похудение' },
    { value: 'health',   label: '💚 Общее здоровье' },
    { value: 'other',    label: '📌 Другое' },
];

// Референсные диапазоны для известных показателей (для линий нормы на графиках).
const REFERENCE_RANGES = {
    'glucose':   { min: 3.9, max: 5.5,  unit: 'ммоль/л' },
    'глюкоза':   { min: 3.9, max: 5.5,  unit: 'ммоль/л' },
    'hemoglobin':{ min: 130, max: 170, unit: 'г/л' },
    'гемоглобин':{ min: 130, max: 170, unit: 'г/л' },
    'cholesterol':{ min: 3.0, max: 5.2, unit: 'ммоль/л' },
    'холестерин':{ min: 3.0, max: 5.2, unit: 'ммоль/л' },
    'creatinine':{ min: 62,  max: 115, unit: 'мкмоль/л' },
    'креатинин': { min: 62,  max: 115, unit: 'мкмоль/л' },
};

const TYPE_LABELS = Object.fromEntries(PROJECT_TYPES.map(t => [t.value, t.label]));
const TYPE_ICON = { course: '💉', diabetes: '🩸', weight: '⚖️', health: '💚', other: '📌' };
const HISTORY_LABELS = { analysis: 'Анализ', bioscan: 'Bioscan', questionnaire: 'Опросник' };

// Состояние SPA.
const state = {
    projects: [],
    detail: null,
    currentProjectId: null,
    historyFilter: '',
    selectedMetrics: new Set(),
    charts: {},
};

// ------------------------------------------------------------
// Утилиты
// ------------------------------------------------------------
function api(path, opts = {}) {
    const initData = getInitData();
    opts.headers = Object.assign({ 'Content-Type': 'application/json' }, opts.headers || {});
    opts.headers['X-Telegram-Init-Data'] = initData;
    const sep = path.includes('?') ? '&' : '?';
    const url = path + sep + 'initData=' + encodeURIComponent(initData);
    return fetch(url, opts).then(r => {
        if (r.status === 401) throw new Error('unauthorized');
        return r.json().then(body => ({ ok: r.ok, status: r.status, body }));
    });
}

function $(id) { return document.getElementById(id); }

function showView(id) {
    ['view-projects', 'view-create', 'view-project'].forEach(v => $(v).classList.add('hidden'));
    $(id).classList.remove('hidden');
}

function setHeader(title, sub) {
    $('headerTitle').textContent = title;
    $('headerSub').textContent = sub || '';
}

function showToast(msg) {
    const t = $('toast');
    t.textContent = msg;
    t.classList.remove('hidden');
    clearTimeout(t._timer);
    t._timer = setTimeout(() => t.classList.add('hidden'), 2600);
}

function showLoading(on) {
    $('loading').classList.toggle('hidden', !on);
}

function escapeHTML(s) {
    return String(s == null ? '' : s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function formatDate(d) {
    if (!d) return '—';
    const dt = new Date(d);
    if (isNaN(dt)) return '—';
    return dt.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

// ------------------------------------------------------------
// Стартовая загрузка
// ------------------------------------------------------------
document.addEventListener('DOMContentLoaded', () => {
    if (tg) { tg.ready(); tg.expand(); }
    initCreateForm();
    initProjectTabs();
    initModal();
    loadProjects();
});

function loadProjects() {
    showLoading(true);
    api('/api/monitoring/projects')
        .then(({ ok, body }) => {
            if (!ok) throw new Error(body.error || 'Ошибка загрузки');
            state.projects = body || [];
            renderProjects();
            setHeader('Мои проекты', 'Отслеживайте показатели во времени');
            showView('view-projects');
        })
        .catch(err => { showToast('Не удалось загрузить проекты: ' + err.message); })
        .finally(() => showLoading(false));
}

// ------------------------------------------------------------
// Список проектов / пустое состояние
// ------------------------------------------------------------
function renderProjects() {
    const list = $('projectsList');
    const empty = $('emptyState');
    list.innerHTML = '';

    if (!state.projects.length) {
        list.classList.add('hidden');
        empty.classList.remove('hidden');
        return;
    }
    list.classList.remove('hidden');
    empty.classList.add('hidden');

    state.projects.forEach(p => {
        const card = document.createElement('div');
        card.className = 'project-card';
        const statusPill = p.status === 'completed'
            ? '<span class="status-pill status-completed">Завершён</span>'
            : '<span class="status-pill status-active">Активен</span>';
        const end = p.end_date ? ' — ' + formatDate(p.end_date) : '';
        card.innerHTML = `
            <div class="pc-top">
                <div class="pc-name">${escapeHTML(p.name)}</div>
                ${statusPill}
            </div>
            <div class="pc-type">${TYPE_ICON[p.type] || '📌'} ${escapeHTML(TYPE_LABELS[p.type] || p.type)}</div>
            <div class="pc-meta">📅 ${formatDate(p.start_date)}${end} · 📎 ${p.entry_ids.length} запис.</div>`;
        card.addEventListener('click', () => openProject(p.id));
        list.appendChild(card);
    });
}

// ------------------------------------------------------------
// Создание проекта
// ------------------------------------------------------------
function initCreateForm() {
    const sel = $('inpType');
    sel.innerHTML = PROJECT_TYPES.map(t => `<option value="${t.value}">${t.label}</option>`).join('');

    const today = new Date().toISOString().slice(0, 10);
    $('inpStart').value = today;

    $('emptyCreateBtn').addEventListener('click', showCreate);
    $('createCancel').addEventListener('click', () => loadProjects());
    $('createSubmit').addEventListener('click', submitCreate);
}

function showCreate() {
    $('inpName').value = '';
    $('inpEnd').value = '';
    $('inpStart').value = new Date().toISOString().slice(0, 10);
    setHeader('Новый проект', 'Заполните параметры мониторинга');
    showView('view-create');
}

function submitCreate() {
    const payload = {
        name: $('inpName').value.trim(),
        type: $('inpType').value,
        start_date: $('inpStart').value,
        end_date: $('inpEnd').value.trim(),
    };
    if (!payload.name) { showToast('Введите название проекта'); return; }
    showLoading(true);
    api('/api/monitoring/projects', { method: 'POST', body: JSON.stringify(payload) })
        .then(({ ok, body }) => {
            if (!ok) throw new Error(body.error || 'Ошибка создания');
            showToast('✅ Проект создан');
            openProject(body.id);
        })
        .catch(err => showToast('Ошибка: ' + err.message))
        .finally(() => showLoading(false));
}

// ------------------------------------------------------------
// Деталь проекта
// ------------------------------------------------------------
function openProject(id) {
    state.currentProjectId = id;
    showLoading(true);
    api('/api/monitoring/projects/' + id)
        .then(({ ok, body }) => {
            if (!ok) throw new Error(body.error || 'Ошибка');
            state.detail = body;
            renderProjectDetail();
            setHeader(body.project.name, TYPE_LABELS[body.project.type] || body.project.type);
            showView('view-project');
            switchTab('info');
        })
        .catch(err => showToast('Не удалось открыть проект: ' + err.message))
        .finally(() => showLoading(false));
}

function renderProjectDetail() {
    const d = state.detail;
    const p = d.project;

    $('projName').textContent = p.name;
    const end = p.end_date ? ' — ' + formatDate(p.end_date) : ' (открытый)';
    $('projMeta').innerHTML = `<span class="tag">${TYPE_ICON[p.type] || '📌'} ${escapeHTML(TYPE_LABELS[p.type] || p.type)}</span>` +
        `<span class="tag">${p.status === 'completed' ? 'Завершён' : 'Активен'}</span>` +
        `<span>📅 ${formatDate(p.start_date)}${end}</span>`;

    // Инфо-карточка
    const completeBtn = p.status === 'active'
        ? `<button class="btn-ghost small" id="completeBtn" style="margin-top:10px;">✅ Завершить проект</button>` : '';
    $('projInfoCard').innerHTML = `
        <h2>Информация о проекте</h2>
        <div class="info-row"><span class="k">Название</span><span class="v">${escapeHTML(p.name)}</span></div>
        <div class="info-row"><span class="k">Тип</span><span class="v">${escapeHTML(TYPE_LABELS[p.type] || p.type)}</span></div>
        <div class="info-row"><span class="k">Дата начала</span><span class="v">${formatDate(p.start_date)}</span></div>
        <div class="info-row"><span class="k">Дата окончания</span><span class="v">${p.end_date ? formatDate(p.end_date) : '—'}</span></div>
        <div class="info-row"><span class="k">Статус</span><span class="v">${p.status === 'completed' ? 'Завершён' : 'Активен'}</span></div>
        <div class="info-row"><span class="k">Привязано записей</span><span class="v">${d.entries.length}</span></div>
        ${completeBtn}`;
    if (p.status === 'active') {
        $('completeBtn').addEventListener('click', () => completeProject(p.id));
    }

    // Мини-список записей в инфо
    const mini = $('projEntriesMini');
    if (!d.entries.length) {
        mini.innerHTML = `<h2>Записи</h2><div class="empty-inline">Пока нет привязанных записей. Откройте вкладку «Записи», чтобы добавить анализ или биоскан из истории.</div>`;
    } else {
        mini.innerHTML = `<h2>Записи (${d.entries.length})</h2><div class="mini-entries">` +
            d.entries.map(e => `<div class="mini-entry"><span>${escapeHTML(e.title || HISTORY_LABELS[e.type] || e.type)}</span><span>${formatDate(e.date)}</span></div>`).join('') +
            `</div>`;
    }

    renderEntries();
    renderMetricPicker();
    renderCharts();
}

// ------------------------------------------------------------
// Вкладка «Записи»
// ------------------------------------------------------------
function renderEntries() {
    const wrap = $('entriesList');
    const none = $('noEntries');
    const d = state.detail;
    wrap.innerHTML = '';
    if (!d.entries.length) {
        none.classList.remove('hidden');
        return;
    }
    none.classList.add('hidden');
    d.entries.forEach(e => {
        const row = document.createElement('div');
        row.className = 'entry-row';
        row.innerHTML = `
            <div class="entry-info">
                <div class="e-title">${escapeHTML(e.title || HISTORY_LABELS[e.type] || e.type)}</div>
                <div class="e-meta">${HISTORY_LABELS[e.type] || e.type} · ${formatDate(e.date)} · ${Object.keys(e.metrics || {}).length} показ.</div>
            </div>
            <button class="entry-remove">Отвязать</button>`;
        row.querySelector('.entry-remove').addEventListener('click', () => unbindEntry(e.id));
        wrap.appendChild(row);
    });
}

function unbindEntry(entryID) {
    showLoading(true);
    api(`/api/monitoring/projects/${state.currentProjectId}/entries/${entryID}`, { method: 'DELETE' })
        .then(({ ok, body }) => {
            if (!ok) throw new Error(body.error || 'Ошибка');
            showToast('Запись отвязана');
            openProject(state.currentProjectId);
        })
        .catch(err => showToast('Ошибка: ' + err.message))
        .finally(() => showLoading(false));
}

function completeProject(id) {
    showLoading(true);
    api(`/api/monitoring/projects/${id}/complete`, { method: 'POST' })
        .then(({ ok, body }) => {
            if (!ok) throw new Error(body.error || 'Ошибка');
            showToast('✅ Проект завершён');
            openProject(id);
        })
        .catch(err => showToast('Ошибка: ' + err.message))
        .finally(() => showLoading(false));
}

// ------------------------------------------------------------
// Выбор показателей + графики
// ------------------------------------------------------------
function renderMetricPicker() {
    const d = state.detail;
    const picker = $('metricPicker');
    picker.innerHTML = '';
    if (!d.available_metrics.length) {
        picker.innerHTML = '<span style="font-size:12px;color:var(--gray)">Нет числовых показателей в привязанных записях.</span>';
        return;
    }
    // По умолчанию выбираем все метрики (но не более 6, чтобы не загромождать).
    if (state.selectedMetrics.size === 0) {
        d.available_metrics.slice(0, 6).forEach(m => state.selectedMetrics.add(m));
    }
    d.available_metrics.forEach(m => {
        const chip = document.createElement('div');
        chip.className = 'metric-chip' + (state.selectedMetrics.has(m) ? ' active' : '');
        chip.textContent = m;
        chip.addEventListener('click', () => {
            if (state.selectedMetrics.has(m)) state.selectedMetrics.delete(m);
            else state.selectedMetrics.add(m);
            renderMetricPicker();
            renderCharts();
        });
        picker.appendChild(chip);
    });
}

function renderCharts() {
    const d = state.detail;
    const wrap = $('chartsWrap');
    const none = $('noCharts');
    // Уничтожаем старые графики
    Object.values(state.charts).forEach(c => { try { c.destroy(); } catch (e) {} });
    state.charts = {};
    wrap.innerHTML = '';

    const metrics = Array.from(state.selectedMetrics).filter(m => d.available_metrics.includes(m));
    if (!metrics.length || !d.entries.length) {
        none.classList.remove('hidden');
        return;
    }
    none.classList.add('hidden');

    metrics.forEach(metric => {
        const box = document.createElement('div');
        box.className = 'chart-box';
        box.innerHTML = `<h3>${escapeHTML(metric)}</h3><div class="chart-area"><canvas></canvas></div>`;
        wrap.appendChild(box);

        const labels = [];
        const values = [];
        d.entries.forEach(e => {
            if (e.metrics && e.metrics[metric] !== undefined) {
                labels.push(formatDate(e.date));
                values.push(e.metrics[metric]);
            }
        });
        if (!values.length) return;

        const ref = findReference(metric);
        const datasets = [{
            label: metric,
            data: values,
            borderColor: '#1FA6A8',
            backgroundColor: 'rgba(31,166,168,0.15)',
            borderWidth: 3,
            fill: true,
            tension: 0.35,
            pointRadius: 5,
            pointHoverRadius: 7,
        }];

        if (ref) {
            datasets.push({
                label: 'Норма (мин)',
                data: labels.map(() => ref.min),
                borderColor: '#4F8A6D',
                borderWidth: 1,
                borderDash: [5, 4],
                pointRadius: 0,
                fill: false,
            });
            datasets.push({
                label: 'Норма (макс)',
                data: labels.map(() => ref.max),
                borderColor: '#E8744A',
                borderWidth: 1,
                borderDash: [5, 4],
                pointRadius: 0,
                fill: false,
            });
        }

        const ctx = box.querySelector('canvas').getContext('2d');
        state.charts[metric] = new Chart(ctx, {
            type: 'line',
            data: { labels, datasets },
            options: {
                responsive: true,
                maintainAspectRatio: false,
                interaction: { intersect: false, mode: 'index' },
                scales: {
                    y: { beginAtZero: false, grid: { color: '#E5EAEA' } },
                    x: { grid: { display: false } },
                },
                plugins: {
                    legend: { position: 'bottom', labels: { boxWidth: 10, padding: 8, font: { size: 10 } } },
                    tooltip: { callbacks: {} },
                },
            },
        });
    });
}

function findReference(metric) {
    const key = String(metric).toLowerCase();
    for (const k in REFERENCE_RANGES) {
        if (key === k || key.includes(k)) return REFERENCE_RANGES[k];
    }
    return null;
}

// ------------------------------------------------------------
// Вкладки проекта
// ------------------------------------------------------------
function initProjectTabs() {
    $('projBack').addEventListener('click', () => { state.detail = null; loadProjects(); });
    document.querySelectorAll('#view-project .tab-btn').forEach(btn => {
        btn.addEventListener('click', () => switchTab(btn.dataset.tab));
    });
}

function switchTab(tab) {
    document.querySelectorAll('#view-project .tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelector(`#view-project .tab-btn[data-tab="${tab}"]`).classList.add('active');
    ['info', 'entries', 'charts'].forEach(t => $('tab-' + t).classList.remove('active'));
    $('tab-' + tab).classList.add('active');
    if (tab === 'charts') {
        // перерисуем графики при показе (нужно для корректных размеров canvas)
        setTimeout(renderCharts, 30);
    }
}

// ------------------------------------------------------------
// Модалка истории (привязка записей)
// ------------------------------------------------------------
let historyPage = 1;
function initModal() {
    $('modalClose').addEventListener('click', closeModal);
    $('modalBackdrop').addEventListener('click', closeModal);
    $('addEntryBtn').addEventListener('click', () => openHistoryModal(''));

    const filters = $('historyFilters');
    const types = [
        { v: '', label: 'Все' },
        { v: 'analysis', label: 'Анализы' },
        { v: 'bioscan', label: 'Bioscan' },
        { v: 'questionnaire', label: 'Опросники' },
    ];
    filters.innerHTML = types.map(t => `<button class="filter-btn${t.v === '' ? ' active' : ''}" data-type="${t.v}">${t.label}</button>`).join('');
    filters.querySelectorAll('.filter-btn').forEach(b => {
        b.addEventListener('click', () => {
            filters.querySelectorAll('.filter-btn').forEach(x => x.classList.remove('active'));
            b.classList.add('active');
            historyPage = 1;
            openHistoryModal(b.dataset.type);
        });
    });
}

function openHistoryModal(type) {
    state.historyFilter = type;
    historyPage = 1;
    $('modal').classList.remove('hidden');
    loadHistory();
}

function closeModal() { $('modal').classList.add('hidden'); }

function loadHistory() {
    const params = new URLSearchParams({ page: historyPage, page_size: 50 });
    if (state.historyFilter) params.set('type', state.historyFilter);
    showLoading(true);
    api('/api/monitoring/history?' + params.toString())
        .then(({ ok, body }) => {
            if (!ok) throw new Error(body.error || 'Ошибка');
            renderHistory(body);
        })
        .catch(err => showToast('Ошибка истории: ' + err.message))
        .finally(() => showLoading(false));
}

function renderHistory(resp) {
    const list = $('historyList');
    const none = $('noHistory');
    list.innerHTML = '';
    const bound = new Set((state.detail && state.detail.project.entry_ids) || []);

    if (!resp.entries.length) {
        none.classList.remove('hidden');
        return;
    }
    none.classList.add('hidden');

    resp.entries.forEach(h => {
        const item = document.createElement('div');
        item.className = 'history-item';
        const isBound = bound.has(h.id);
        item.innerHTML = `
            <div>
                <div class="h-title">${escapeHTML(h.title || HISTORY_LABELS[h.type] || h.type)}</div>
                <div class="h-meta">${HISTORY_LABELS[h.type] || h.type} · ${formatDate(h.date)}</div>
            </div>
            <button ${isBound ? 'disabled' : ''}>${isBound ? '✓ Привязано' : 'Добавить'}</button>`;
        if (!isBound) {
            item.querySelector('button').addEventListener('click', () => bindEntry(h.id));
        }
        list.appendChild(item);
    });
}

function bindEntry(entryID) {
    showLoading(true);
    api(`/api/monitoring/projects/${state.currentProjectId}/entries`, {
        method: 'POST',
        body: JSON.stringify({ entry_id: entryID }),
    })
        .then(({ ok, body }) => {
            if (!ok) throw new Error(body.error || 'Ошибка');
            showToast('✅ Запись добавлена');
            // обновляем деталь, чтобы кнопка стала disabled
            openProject(state.currentProjectId);
        })
        .catch(err => showToast('Ошибка: ' + err.message))
        .finally(() => showLoading(false));
}
