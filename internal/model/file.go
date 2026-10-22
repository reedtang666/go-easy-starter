package model

import view "github.com/reedtang666/go-easy-starter/internal/model/view"

type FileResult struct {
	*view.Attachment
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
}
