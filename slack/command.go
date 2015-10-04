package slack

type OutCommand struct {
	Type      string `json:"type"`
	TeamId    string `json:"team_id"`
	UserId    string `json:"user_id"`
	ChannelId string `json:"channel_id"`
	Payload   string `json:"payload"`
}

type Payload struct {
	Message  string `json:"message"`
	ImageUrl string `json:"image_url"`
}

type Attachment struct {
	ImageUrl string `json:"image_url"`
	Pretext  string `json:"pretext"`
}
