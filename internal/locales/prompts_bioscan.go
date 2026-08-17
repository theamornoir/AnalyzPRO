package locales

// PromptForBioscan - промпт для bioscan JSON.
func PromptForBioscan(contextInfo string) string {
	return `Ты анализируешь фото тела для фитнес отчёта.

Верни ТОЛЬКО JSON.
Без markdown.
Без комментариев.

Формат:

{
  "score": 85,
  "level": "Хорошая форма",
  "summary": "описание",
  "body": {
    "height": "",
    "weight": "",
    "muscle_mass": "",
    "fat": ""
  },
  "composition": "",
  "profile": {
    "composition": 80,
    "muscle_development": 75,
    "balance": 85,
    "potential": 90
  },
  "zones": [
    {
      "name": "",
      "score": 80,
      "status": "",
      "description": ""
    }
  ],
  "muscles": [
    {
      "name": "",
      "level": "",
      "assessment": "",
      "symmetry": "",
      "recommendation": ""
    }
  ],
  "posture": {
    "type": "",
    "head": "",
    "shoulders": "",
    "pelvis": "",
    "description": ""
  },
  "attention_zones": [
    {
      "name": "",
      "problem": "",
      "solution": ""
    }
  ],
  "priorities": [
    {
      "title": "",
      "description": ""
    }
  ],
  "training_days": [
    {
      "day": "",
      "exercises": [
        {
          "name": "",
          "sets": "",
          "reps": ""
        }
      ]
    }
  ],
  "nutrition": [],
  "recovery": [],
  "progress": {
    "recheck": "",
    "targets": []
  }
}

Данные пользователя:
` + contextInfo
}
