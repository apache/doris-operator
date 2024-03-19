package sub_controller

// define event type for sub controller, Type  can be one of Normal, Warning.
const (
	EventNormal  = "Normal"
	EventWarning = "Warning"
)

// 'reason' should be short and unique; it should be in UpperCamelCase format (starting with a capital letter).
const (
	StatefulSetNotExist     = "StatefulSetNotExist"
	AutoScalerDeleteFailed  = "AutoScalerDeleteFailed"
	ComponentImageUpdate    = "ComponentImageUpdate"
	PVCListFailed           = "PVCListFailed"
	PVCUpdate               = "PVCUpdated"
	PVCUpdateFailed         = "PVCUpdateFailed"
	PVCCreate               = "PVCCreate"
	PVCCreateFailed         = "PVCCreateFailed"
	FollowerScaleDownFailed = "FollowerScaleDownFailed"
)
