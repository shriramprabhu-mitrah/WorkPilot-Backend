package response

type SuccessResponse struct {
	Success    bool        `json:"success"`
	StatusCode int         `json:"status_code"`
	Message    string      `json:"message"`
	Data       any         `json:"data,omitempty"`
	Meta       *Pagination `json:"meta,omitempty"`
}

type Pagination struct {
	Page        int  `json:"page"`
	PageSize    int  `json:"page_size"`
	TotalItems  int  `json:"total_items"`
	TotalPages  int  `json:"total_pages"`
	HasNext     bool `json:"has_next"`
	HasPrevious bool `json:"has_previous"`
}
