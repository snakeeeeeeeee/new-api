package model

type CanvasGrant struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:uk_canvas_grant_user_client,priority:1"`
	ClientId     string `json:"client_id" gorm:"size:64;not null;uniqueIndex:uk_canvas_grant_user_client,priority:2"`
	ImageTokenId int    `json:"image_token_id" gorm:"not null;default:0"`
	VideoTokenId int    `json:"video_token_id" gorm:"not null;default:0"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime  int64  `json:"updated_time" gorm:"bigint"`
}

type CanvasAuthorizationCode struct {
	Id            int64  `json:"id"`
	CodeHash      string `json:"-" gorm:"size:64;not null;uniqueIndex"`
	UserId        int    `json:"user_id" gorm:"not null;index"`
	ClientId      string `json:"client_id" gorm:"size:64;not null"`
	RedirectUri   string `json:"redirect_uri" gorm:"type:text;not null"`
	CodeChallenge string `json:"-" gorm:"size:128;not null"`
	ConfigHash    string `json:"-" gorm:"size:64;not null"`
	ExpiredAt     int64  `json:"expired_at" gorm:"bigint;not null;index"`
	ConsumedAt    int64  `json:"consumed_at" gorm:"bigint;not null;default:0;index"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint;not null"`
}
