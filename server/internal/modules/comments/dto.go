package comments

// CreateCommentRequest is the JSON body for POST /pins/:id/comments.
type CreateCommentRequest struct {
	Body string `json:"body" binding:"required"`
}
