package openyouku

type ResponseError struct {
	Code      int    `json:"code"`
	Provider  string `json:"provider"`
	Desc      string `json:"desc"`
	ErrorCode int    `json:"error_code"`
	ErrorMsg  string `json:"error_msg"`
}

type Response struct {
	Error ResponseError `json:"e"`
	Cost  float64       `json:"cost"`
	Data  interface{}   `json:"data"`
}

type TokenResponse struct {
	Errno   int     `json:"errno"`
	ErrText string  `json:"errText"`
	Cost    float64 `json:"cost"`
	Token   struct {
		ExpireTime   int    `json:"expireTime"`
		RexpireTime  int    `json:"rexpireTime"`
		StartTime    int64  `json:"startTime"`
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		OpenId       string `json:"openId"`
	} `json:"token"`
}

type OssParam struct {
	Endpoint         string `json:"endpoint"`
	ExpireTime       string `json:"expire_time"`
	OssBucket        string `json:"oss_bucket"`
	OssObject        string `json:"oss_object"`
	SecurityToken    string `json:"security_token"`
	TempAccessID     string `json:"temp_access_id"`
	TempAccessSecret string `json:"temp_access_secret"`
	UploadToken      string `json:"upload_token"`
	Vid              string `json:"vid"`
}

type UploadResponse struct {
	Error ResponseError `json:"e"`
	Cost  float64       `json:"cost"`
	Data  []OssParam    `json:"data"`
}

type VideoData struct {
	Vid         string `json:"vid"`
	UploadToken string `json:"upload_token"`
	OwOssBucket string `json:"ow_oss_bucket,omitempty"`
	OwOssObject string `json:"ow_oss_object,omitempty"`
}

type VideoResponse struct {
	Error ResponseError `json:"e"`
	Cost  float64       `json:"cost"`
	Data  []VideoData   `json:"data"`
}
