package httpmodel

// ResponseModel 通用响应基础模型
type ResponseModel struct {
	ID          uint   `json:"id" copy:"ID"`
	CreatedTime string `json:"createdTime" copy:"createdAt"`
	UpdatedTime string `json:"updatedTime" copy:"updatedAt"`
}
