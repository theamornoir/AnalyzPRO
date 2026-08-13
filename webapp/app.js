// Аналитика дашборда AnalyzPRO
document.addEventListener('DOMContentLoaded', () => {
    loadDashboard();
});

async function loadDashboard() {
    try {
        const response = await fetch('/api/metrics');
        if (!response.ok) throw new Error('Failed to load metrics');
        
        const data = await response.json();
        renderDashboard(data);
    } catch (error) {
        console.error('Ошибка загрузки дашборда:', error);
        loadMockData();
    }
}

function loadMockData() {
    const mockData = {
        healthIndex: 78,
        energyLevel: 'Высокий',
        analysisDate: new Date().toLocaleDateString('ru-RU'),
        blood: { hemoglobin: 145, leukocytes: 6.2, platelets: 250 },
        nutrition: { protein: 85, carbs: 70, fat: 60 },
        activity: { steps: 8500, calories: 2200, water: 2.5 },
        trend: {
            labels: ['Янв', 'Фев', 'Мар', 'Апр', 'Май', 'Июн'],
            values: [65, 68, 72, 70, 75, 78]
        },
        recommendations: [
            'Увеличить потребление белка до 1.5г/кг массы тела',
            'Добавить 30 минут кардио 3 раза в неделю',
            'Контролировать уровень витамина D',
            'Нормализовать режим сна (7-8 часов)'
        ]
    };
    renderDashboard(mockData);
}

function renderDashboard(data) {
    // Статус
    document.getElementById('healthIndex').textContent = data.healthIndex || '--';
    document.getElementById('energyLevel').textContent = data.energyLevel || '--';
    document.getElementById('analysisDate').textContent = data.analysisDate || '--';
    
    // Круговые диаграммы
    createCircularChart('chartBlood', [
        { label: 'Гемоглобин', value: data.blood?.hemoglobin || 0 },
        { label: 'Лейкоциты', value: data.blood?.leukocytes || 0 },
        { label: 'Тромбоциты', value: data.blood?.platelets || 0 }
    ], ['#1FA6A8', '#4F8A6D', '#73D2D4']);
    
    createCircularChart('chartNutrition', [
        { label: 'Белки', value: data.nutrition?.protein || 0 },
        { label: 'Углеводы', value: data.nutrition?.carbs || 0 },
        { label: 'Жиры', value: data.nutrition?.fat || 0 }
    ], ['#E8744A', '#C9A84C', '#1FA6A8']);
    
    createCircularChart('chartActivity', [
        { label: 'Шаги', value: (data.activity?.steps || 0) / 100 },
        { label: 'Калории', value: (data.activity?.calories || 0) / 50 },
        { label: 'Вода', value: (data.activity?.water || 0) * 40 }
    ], ['#3B82A0', '#4F8A6D', '#1FA6A8']);
    
    // Динамика
    createTrendChart(data.trend);
    
    // Рекомендации
    const recList = document.getElementById('recList');
    recList.innerHTML = '';
    (data.recommendations || []).forEach(rec => {
        const li = document.createElement('li');
        li.textContent = rec;
        recList.appendChild(li);
    });
    
    // Время обновления
    document.getElementById('lastUpdate').textContent = 
        'Обновлено: ' + new Date().toLocaleTimeString('ru-RU');
}

function createCircularChart(canvasId, datasets, colors) {
    const ctx = document.getElementById(canvasId).getContext('2d');
    
    new Chart(ctx, {
        type: 'doughnut',
        data: {
            labels: datasets.map(d => d.label),
            datasets: [{
                data: datasets.map(d => d.value),
                backgroundColor: colors,
                borderWidth: 0,
                hoverOffset: 4
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            cutout: '65%',
            plugins: {
                legend: {
                    position: 'bottom',
                    labels: {
                        boxWidth: 12,
                        padding: 10,
                        font: { size: 10 }
                    }
                }
            }
        }
    });
}

function createTrendChart(trendData) {
    const ctx = document.getElementById('chartTrend').getContext('2d');
    
    new Chart(ctx, {
        type: 'line',
        data: {
            labels: trendData.labels || [],
            datasets: [{
                label: 'Индекс здоровья',
                data: trendData.values || [],
                borderColor: '#1FA6A8',
                backgroundColor: 'rgba(31, 166, 168, 0.1)',
                borderWidth: 3,
                fill: true,
                tension: 0.4,
                pointBackgroundColor: '#1FA6A8',
                pointBorderColor: '#fff',
                pointBorderWidth: 2,
                pointRadius: 5
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            scales: {
                y: {
                    beginAtZero: false,
                    min: 0,
                    max: 100,
                    grid: { color: '#E5EAEA' }
                },
                x: {
                    grid: { display: false }
                }
            },
            plugins: {
                legend: { display: false }
            }
        }
    });
}
