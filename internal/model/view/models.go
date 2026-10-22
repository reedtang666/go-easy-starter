package view

import "time"

type User struct {
	Id                int64     `json:"id"`
	Username          string    `json:"username"`
	PasswordHash      string    `json:"-"`
	Nickname          string    `json:"nickname"`
	AvatarId          int64     `json:"avatarId"`
	Status            int       `json:"status"`
	CredentialVersion int64     `json:"-"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
}
type Attachment struct {
	Id          int64     `json:"id"`
	OwnerId     int64     `json:"-"`
	StorageName string    `json:"storageName"`
	Driver      string    `json:"driver"`
	Bucket      string    `json:"-"`
	ObjectKey   string    `json:"-"`
	Name        string    `json:"name"`
	MimeType    string    `json:"mimeType"`
	Size        int64     `json:"size"`
	Digest      string    `json:"digest"`
	Private     bool      `json:"private"`
	CreatedAt   time.Time `json:"createdAt"`
}
type ExampleItem struct {
	Id        int64     `json:"id"`
	OwnerId   int64     `json:"-"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	TaskRuns  int       `json:"taskRuns"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
