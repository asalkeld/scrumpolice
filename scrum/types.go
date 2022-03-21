package scrum

type TeamConfig struct {
	Name               string   `json:"name"`
	Channel            string   `json:"channel"`
	Members            []string `json:"members"`
	Questions          []string `json:"questions"`
	ReportScheduleCron string   `json:"reportScheduleCron"`
	Timezone           string   `json:"timezone"`
	SplitReport        bool     `json:"splitReport"`
}

type Config struct {
	Teams []TeamConfig `json:"teams"`
}

type UserState struct {
	User           string            `json:"user"`
	GithubUser     string            `json:"githubUser"`
	OutOfOffice    bool              `json:"outOfOffice"`
	Started        bool              `json:"started"`
	Skipped        bool              `json:"skipped"`
	LastAnswerDate string            `json:"lastAnswerDate"`
	Answers        map[string]string `json:"answers"`
}

type Report struct {
	Team    string            `json:"team"`
	Date    string            `json:"date"`
	Answers map[string]string `json:"answers"`
}
