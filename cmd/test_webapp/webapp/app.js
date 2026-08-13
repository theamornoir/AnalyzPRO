// ============================================
// AnalyzPRO Web App — Основной файл
// ============================================
// Интерактивные графики, вкладки, фильтры
// ============================================

let charts = {};

document.addEventListener('DOMContentLoaded', () => {
    initDashboard();
    initTabs();
    initFilters();
});

function initDashboard() {
    const data = MOCK;
    
    // Заголовок
    document.getElementById('userName').textContent = data.user.name;
    document.getElementById('userAge').textContent = data.user.age + ' лет';
    document.getElementById('premiumBadge').textContent = '💎 Premium';
    document.getElementById('premiumBadge').className = 'premium-badge';
    
    // Индексы
    document.getElementById('healthIndex').textContent = data.health.overall;
    document.getElementById('energyLevel').textContent = data.health.energy + '%';
    document.getElementById('stressLevel').textContent = data.health.stress + '%';
    document.getElementById('sleepHours').textContent = data.health.sleep + 'ч';
    document.getElementById('waterLiters').textContent = data.health.hydration/10 + 'л';
    
    // Цвет индекса
    document.getElementById('healthIndex').style.color = 
        data.health.overall >= 80 ? '#1FA6A8' : 
        data.health.overall >= 60 ? '#E8744A' : '#D32F2F';
    
    // Инициализация всех графиков
    createHealthDoughnut();
    createBloodRadar();
    createNutritionBar();
    createTrendLine();
    createMuscleBars();
    createActivityGauge();
    
    // Активная вкладка по умолчанию
    switchTab('overview');
}

// ============================================
// ВКЛАДКИ
// ============================================
function initTabs() {
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            const tabId = btn.dataset.tab;
            switchTab(tabId);
        });
    });
}

function switchTab(tabId) {
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
    
    document.querySelector(`[data-tab="${tabId}"]`).classList.add('active');
    document.getElementById('tab-' + tabId).classList.add('active');
    
    if (charts.trendLine) charts.trendLine.resize();
    if (charts.bloodRadar) charts.bloodRadar.resize();
}

// ============================================
// ФИЛЬТРЫ
// ============================================
function initFilters() {
    document.querySelectorAll('.filter-btn').forEach(btn => {
        btn.addEventListener('click', () => {
            document.querySelectorAll('.filter-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            
            const period = btn.dataset.period;
            updateTrendForPeriod(period);
        });
    });
}

function updateTrendForPeriod(period) {
    const data = MOCK;
    let labels, healthData, energyData;
    
    switch(period) {
        case 'month':
            labels = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];
            healthData = [76, 77, 75, 78, 79, 80, 78];
            energyData = [80, 82, 79, 83, 84, 85, 82];
            break;
        case 'year':
            labels = ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн', 'Июл', 'Авг', 'Сен', 'Окт', 'Ноя', 'Дек'];
            healthData = [60, 62, 63, 65, 68, 70, 72, 70, 73, 75, 76, 78];
            energyData = [65, 67, 68, 70, 72, 74, 75, 78, 76, 78, 80, 82];
            break;
        default:
            labels = data.trend.labels;
            healthData = data.trend.health;
            energyData = data.trend.energy;
    }
    
    charts.trendLine.data.labels = labels;
    charts.trendLine.data.datasets[0].data = healthData;
    charts.trendLine.data.datasets[1].data = energyData;
    charts.trendLine.update('none');
}

// ============================================
// ГРАФИКИ
// ============================================
function createHealthDoughnut() {
    const ctx = document.getElementById('chartHealth').getContext('2d');
    charts.health = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: ['Здоровье', 'Энергия', 'Стресс↓', 'Сон', 'Вода'],
            datasets: [{
                data: [MOCK.health.overall, MOCK.health.energy, 100-MOCK.health.stress, MOCK.health.sleep, MOCK.health.hydration],
                backgroundColor: ['#1FA6A8', '#4F8A6D', '#E8744A', '#3B82A0', '#C9A84C'],
                borderWidth: 0,
                hoverOffset: 8
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            cutout: '70%',
            plugins: {
                legend: { position: 'bottom', labels: { boxWidth: 10, padding: 8, font: { size: 10 } } }
            }
        }
    });
}

function createBloodRadar() {
    const ctx = document.getElementById('chartBlood').getContext('2d');
    const b = MOCK.blood;
    charts.bloodRadar = new Chart(ctx, {
        type: 'radar',
        data: {
            labels: ['Hb', 'WBC', 'PLT', 'Glu', 'Chol', 'Cr'],
            datasets: [{
                label: 'Ваш уровень',
                data: [b.hemoglobin.score, b.leukocytes.score, b.platelets.score, b.glucose.score, b.cholesterol.score, b.crreatinine.score],
                backgroundColor: 'rgba(31, 166, 168, 0.2)',
                borderColor: '#1FA6A8',
                borderWidth: 2,
                pointBackgroundColor: '#1FA6A8'
            }, {
                label: 'Норма',
                data: [90, 90, 90, 90, 90, 90],
                backgroundColor: 'rgba(79, 138, 109, 0.1)',
                borderColor: '#4F8A6D',
                borderWidth: 1,
                borderDash: [5, 5],
                pointRadius: 0
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                r: { beginAtZero: true, max: 100, ticks: { stepSize: 25, font: { size: 8 } } }
            },
            plugins: {
                legend: { position: 'bottom', labels: { boxWidth: 10, padding: 8, font: { size: 10 } } }
            }
        }
    });
}

function createNutritionBar() {
    const ctx = document.getElementById('chartNutrition').getContext('2d');
    const n = MOCK.nutrition;
    charts.nutritionBar = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: ['Белки', 'Жиры', 'Углеводы', 'Калории', 'Вода', 'Клетчатка'],
            datasets: [{
                label: 'Текущее',
                data: [n.protein.current, n.fats.current, n.carbs.current, n.calories.current, n.water.current*40, n.fiber.current],
                backgroundColor: '#1FA6A8',
                borderRadius: 4
            }, {
                label: 'Цель',
                data: [n.protein.target, n.fats.target, n.carbs.target, n.calories.target, n.water.target*40, n.fiber.target],
                backgroundColor: '#E5EAEA',
                borderRadius: 4
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: { beginAtZero: true, grid: { color: '#E5EAEA' } },
                x: { grid: { display: false } }
            },
            plugins: {
                legend: { position: 'bottom', labels: { boxWidth: 10, padding: 8, font: { size: 10 } } }
            }
        }
    });
}

function createTrendLine() {
    const ctx = document.getElementById('chartTrend').getContext('2d');
    const t = MOCK.trend;
    const gradient = ctx.createLinearGradient(0, 0, 0, 250);
    gradient.addColorStop(0, 'rgba(31, 166, 168, 0.3)');
    gradient.addColorStop(1, 'rgba(31, 166, 168, 0.02)');
    
    charts.trendLine = new Chart(ctx, {
        type: 'line',
        data: {
            labels: t.labels,
            datasets: [{
                label: 'Здоровье',
                data: t.health,
                borderColor: '#1FA6A8',
                backgroundColor: gradient,
                borderWidth: 3,
                fill: true,
                tension: 0.4,
                pointRadius: 5,
                pointHoverRadius: 7
            }, {
                label: 'Энергия',
                data: t.energy,
                borderColor: '#4F8A6D',
                backgroundColor: 'transparent',
                borderWidth: 2,
                borderDash: [5, 3],
                tension: 0.4,
                pointRadius: 3
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: { intersect: false, mode: 'index' },
            scales: {
                y: { min: 50, max: 100, grid: { color: '#E5EAEA' } },
                x: { grid: { display: false } }
            },
            plugins: {
                legend: { position: 'bottom', labels: { boxWidth: 10, padding: 8, font: { size: 10 } } }
            }
        }
    });
}

function createMuscleBars() {
    const ctx = document.getElementById('chartMuscles').getContext('2d');
    const sorted = [...MOCK.muscles].sort((a, b) => b.score - a.score);
    const colors = sorted.map(m => {
        if (m.score >= 80) return '#1FA6A8';
        if (m.score >= 65) return '#4F8A6D';
        if (m.score >= 50) return '#C9A84C';
        return '#E8744A';
    });
    
    charts.muscleBars = new Chart(ctx, {
        type: 'bar',
        data: {
            labels: sorted.map(m => m.name),
            datasets: [{
                data: sorted.map(m => m.score),
                backgroundColor: colors,
                borderRadius: 4,
                barThickness: 16
            }]
        },
        options: {
            indexAxis: 'y',
            responsive: true,
            maintainAspectRatio: false,
            scales: { x: { min: 0, max: 100, grid: { color: '#E5EAEA' } }, y: { grid: { display: false } } },
            plugins: { legend: { display: false } }
        }
    });
}

function createActivityGauge() {
    const ctx = document.getElementById('chartActivity').getContext('2d');
    const a = MOCK.activity;
    charts.activityGauge = new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: ['Шаги', 'Калории', 'Тренировка', 'Сон'],
            datasets: [{
                data: [
                    Math.min(a.steps.current/a.steps.target*100, 100),
                    Math.min(a.calories.current/a.calories.target*100, 100),
                    Math.min(a.workoutMin.current/a.workoutMin.target*100, 100),
                    Math.min(a.sleepHours.current/a.sleepHours.target*100, 100)
                ],
                backgroundColor: ['#1FA6A8', '#4F8A6D', '#3B82A0', '#C9A84C'],
                borderWidth: 0,
                circumference: 180,
                rotation: 270
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            cutout: '60%',
            plugins: { legend: { position: 'bottom', labels: { boxWidth: 10, padding: 8, font: { size: 10 } } } }
        }
    });
}

// ============================================
// ДОПОЛНИТЕЛЬНЫЕ ЭЛЕМЕНТЫ
// ============================================
const recList = document.getElementById('recList');
MOCK.recommendations.forEach(rec => {
    const li = document.createElement('li');
    li.textContent = rec;
    recList.appendChild(li);
});

const riskList = document.getElementById('riskList');
MOCK.risks.forEach(risk => {
    const div = document.createElement('div');
    div.className = 'risk-item';
    if (risk.level === 'critical') div.classList.add('risk-critical');
    else if (risk.level === 'warning') div.classList.add('risk-warning');
    
    const icon = risk.level === 'critical' ? '🔴' : '🟡';
    div.innerHTML = `
        <div class="risk-header">
            <span class="risk-name">${icon} ${risk.name}</span>
            <span class="risk-level">${risk.level === 'critical' ? 'Критично' : 'Внимание'}</span>
        </div>
        <p class="risk-desc">${risk.desc}</p>
    `;
    riskList.appendChild(div);
});

const muscleList = document.getElementById('muscleList');
MOCK.muscles.sort((a, b) => b.score - a.score).forEach(m => {
    const div = document.createElement('div');
    div.className = 'muscle-item';
    div.innerHTML = `
        <div class="muscle-header">
            <span class="muscle-name">${m.name}</span>
            <span class="muscle-score">${m.score}%</span>
        </div>
        <div class="muscle-bar-bg">
            <div class="muscle-bar" style="width: ${m.score}%;"></div>
        </div>
        <span class="muscle-status">${m.status}</span>
    `;
    muscleList.appendChild(div);
});
