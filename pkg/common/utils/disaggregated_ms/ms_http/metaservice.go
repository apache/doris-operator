package ms_http

type MSResponse struct {
	Code   string                 `json:"code,omitempty"`
	Msg    string                 `json:"msg,omitempty"`
	Result map[string]interface{} `json:"result,omitempty"`
}

const (
	SuccessCode    string = "OK"
	ALREADY_EXIST  string = "ALREADY_EXISTED"
	NotFound       string = "NOT_FOUND"
	INTERNAL_ERROR string = "INTERNAL_ERROR"
)
