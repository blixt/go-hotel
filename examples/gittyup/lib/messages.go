package lib

type JoinMessage struct {
	User *UserMetadata `json:"user"`
}

func (m JoinMessage) Type() string {
	return "join"
}

type LeaveMessage struct {
}

func (m LeaveMessage) Type() string {
	return "leave"
}

type ChatMessage struct {
	Content string `json:"content"`
}

func (m ChatMessage) Type() string {
	return "chat"
}

type WelcomeMessage struct {
	Users         []*UserMetadata `json:"users"`
	RepoHash      string          `json:"repoHash"`
	CurrentCommit string          `json:"currentCommit"`
	Files         []string        `json:"files"`
}

func (m WelcomeMessage) Type() string {
	return "welcome"
}
