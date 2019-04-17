package apimodel

import "fmt"

type PhotoObj struct {
	PhotoId            string `json:"photoId"`
	PhotoHidden        bool   `json:"photoHidden"`
	PhotoReported      bool   `json:"photoReported"`
	WasModeratedBefore bool   `json:"wasModeratedBefore"`
	BlockReasons       []int  `json:"blockReasons"`
	Likes              int    `json:"likes"`
	UpdatedAt          int64  `json:"updatedAt"`
	S3Key              string `json:"s3Key"`
	PhotoUrl           string `json:"photoUrl"`
	OnlyOwnerCanSee    bool   `json:"onlyOwnerCanSee"`
}

func (req PhotoObj) String() string {
	return fmt.Sprintf("%#v", req)
}


type ProfileObj struct {
	UserId                   string     `json:"userId"`
	Photos                   []PhotoObj `json:"photos"`
	Rows                     []Row      `json:"rows"`
	HowManyPhotosWereBlocked int        `json:"howManyPhotosWereBlocked"`
}

func (req ProfileObj) String() string {
	return fmt.Sprintf("%#v", req)
}


type Row struct {
	Photos []PhotoObj `json:"photosRows"`
}

func (req Row) String() string {
	return fmt.Sprintf("%#v", req)
}

type ModerationResp struct {
	Profiles []ProfileObj `json:"profiles"`
}

func (req ModerationResp) String() string {
	return fmt.Sprintf("%#v", req)
}

type ModerationReq struct {
	QueryType       string            `json:"queryType"`
	Limit           int               `json:"limit"`
	ProfilePhotoMap map[string]string `json:"profilePhotoMap"`
}

func (req ModerationReq) String() string {
	return fmt.Sprintf("%#v", req)
}

type ReportedFormData struct {
	Profiles     []ProfileObj
	ShowData     bool
	Message      string
	SubmitAction string
}

func (req ReportedFormData) String() string {
	return fmt.Sprintf("%#v", req)
}

