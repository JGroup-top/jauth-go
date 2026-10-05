package jauth

type User struct {
	ID       string `json:"sub"`
	Username string `json:"preferred_username"`
	Email    string `json:"email"`
}
