package bot

// Vacancy описывает вакансию
type Vacancy struct {
	ID          string
	Title       string
	Emoji       string
	Description string // краткое описание для промпта
}

// Vacancies — список доступных вакансий
var Vacancies = []Vacancy{
	{
		ID:          "backend",
		Title:       "Backend Developer",
		Emoji:       "⚙️",
		Description: "Go, Python, Java или Node.js. REST/gRPC API, базы данных (SQL/NoSQL), Docker, микросервисы.",
	},
	{
		ID:          "frontend",
		Title:       "Frontend Developer",
		Emoji:       "🖥️",
		Description: "React/Vue/Angular, TypeScript, HTML/CSS, адаптивная вёрстка, производительность UI.",
	},
	{
		ID:          "devops",
		Title:       "DevOps Engineer",
		Emoji:       "🔧",
		Description: "CI/CD (GitLab CI, GitHub Actions), Kubernetes, Docker, Terraform, мониторинг (Prometheus/Grafana), облака (AWS/GCP/Azure).",
	},
	{
		ID:          "sysadmin",
		Title:       "Системный администратор",
		Emoji:       "🖧",
		Description: "Linux/Windows Server, сети (TCP/IP, DNS, VPN), Active Directory, резервное копирование, мониторинг серверов.",
	},
	{
		ID:          "uiux",
		Title:       "UI/UX Designer",
		Emoji:       "🎨",
		Description: "Figma, прототипирование, UX-исследования, дизайн-системы, анимация интерфейсов, Accessibility.",
	},
	{
		ID:          "security",
		Title:       "Кибербезопасность",
		Emoji:       "🔒",
		Description: "Пентестинг, анализ уязвимостей, SIEM, сетевая безопасность, OWASP, сертификаты (CEH, OSCP, CISSP).",
	},
	{
		ID:          "mobile",
		Title:       "Mobile Developer",
		Emoji:       "📱",
		Description: "Android (Kotlin/Java) или iOS (Swift/Objective-C), Flutter/React Native, работа с API, публикация в сторах.",
	},
	{
		ID:          "data",
		Title:       "Data Engineer / Analyst",
		Emoji:       "📊",
		Description: "Python, SQL, ETL-пайплайны, Spark, аналитика данных, визуализация (Tableau/PowerBI/Grafana).",
	},
	{
		ID:          "qa",
		Title:       "QA Engineer",
		Emoji:       "🧪",
		Description: "Ручное и автоматизированное тестирование, Selenium/Playwright/Cypress, нагрузочное тестирование, баг-репорты.",
	},
}

// FindVacancy возвращает вакансию по ID
func FindVacancy(id string) (Vacancy, bool) {
	for _, v := range Vacancies {
		if v.ID == id {
			return v, true
		}
	}
	return Vacancy{}, false
}