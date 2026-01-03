package main

type ProfileConfig struct {
	BaseURL string
	Service string
	UserID  string
}

type Post struct {
	Id        string `json:"id"`
	User      string `json:"user"`
	Service   string `json:"service"`
	Title     string `json:"title"`
	Substring string `json:"substring"`
	Published string `json:"published"`
	File      struct {
	} `json:"file"`
	Attachments []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	} `json:"attachments"`
}

type DetailedPostResponse struct {
	Post        map[string]interface{} `json:"post"`
	Attachments []interface{}          `json:"attachments"`
	Previews    []interface{}          `json:"previews"`
	Videos      []interface{}          `json:"videos"`
	Props       map[string]interface{} `json:"props"`
}

type ProfileResponse struct {
	Id         string      `json:"id"`
	Name       string      `json:"name"`
	Service    string      `json:"service"`
	Indexed    string      `json:"indexed"`
	Updated    string      `json:"updated"`
	PublicId   string      `json:"public_id"`
	RelationId interface{} `json:"relation_id"`
	PostCount  int         `json:"post_count"`
	DmCount    int         `json:"dm_count"`
	ShareCount int         `json:"share_count"`
	ChatCount  int         `json:"chat_count"`
}

type FailedItem struct {
	Post string   `json:"post"`
	URLs []string `json:"urls"`
}
