package middleware

import "TodoList/pkg/jwt"

type Middleware struct {
	jwtManager *jwt.Manager
}

func NewMiddleware(jwtManager *jwt.Manager) *Middleware {
	return &Middleware{
		jwtManager: jwtManager,
	}
}
