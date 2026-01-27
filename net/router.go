package net

type group struct {
	prefix   string
	handlers map[string]HandlerFunc
}

type HandlerFunc func()

type router struct {
	groups []*group
}
