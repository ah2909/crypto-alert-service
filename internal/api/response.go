package api

import "github.com/gin-gonic/gin"

//	success: true  -> { "success": true,  "data": <payload> }
//	success: false -> { "success": false, "error": "<message>" }
type response struct {
	Success bool `json:"success"`
	Data    any `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// respondData writes a success envelope wrapping data.
func respondData(c *gin.Context, status int, data any) {
	c.JSON(status, response{Success: true, Data: data})
}

// respondError writes a failure envelope with a human-readable message.
func respondError(c *gin.Context, status int, msg string) {
	c.JSON(status, response{Success: false, Error: msg})
}
