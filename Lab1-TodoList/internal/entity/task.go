package entity

type Task struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	Priority      string `json:"priority,omitempty"`
	CreatedUserID string `json:"created_user_id"`
}
