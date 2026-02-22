package http

type ErrorResponse struct {
	Error string `json:"error"`
}

type ListResponse[T any] struct {
	Data       []T     `json:"data"`
	NextCursor *string `json:"next_cursor,omitempty"`
}
