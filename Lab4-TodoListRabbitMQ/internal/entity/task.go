package entity

type Task struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description,omitempty"`
	Status        string `json:"status,omitempty"`
	Priority      string `json:"priority,omitempty"`
	CreatedUserID string `json:"created_user_id"`
}
