package sub_controller

// define event type for sub controller, Type  can be one of Normal, Warning.
type EventType string

// only Normal Warning, not add new type.
var (
	EventNormal  EventType = "Normal"
	EventWarning EventType = "Warning"
)

// 'reason' should be short and unique; it should be in UpperCamelCase format (starting with a capital letter).
const (
	StatefulSetNotExist     = "StatefulSetNotExist"
	AutoScalerDeleteFailed  = "AutoScalerDeleteFailed"
	ComponentImageUpdate    = "ComponentImageUpdate"
	PVCListFailed           = "PVCListFailed"
	PVCUpdate               = "PVCUpdated"
	PVCUpdateFailed         = "PVCUpdateFailed"
	PVCDeleteFailed         = "PVCDeleteFailed"
	PVCCreate               = "PVCCreate"
	PVCCreateFailed         = "PVCCreateFailed"
	FollowerScaleDownFailed = "FollowerScaleDownFailed"
)

type EventReason string

var (
	ImageFormatError                  EventReason = "ImageFormatError"
	FDBSpecEmpty                      EventReason = "SpecEmpty"
	FDBAvailableButUnhealth           EventReason = "FDBAvailableButUnhealth"
	FESpecSetError                    EventReason = "FESpecSetError"
	FECreateResourceFailed            EventReason = "FECreateResourceFailed"
	FEApplyResourceFailed             EventReason = "FEApplyResourceFailed"
	FEStatefulsetDeleteFailed         EventReason = "FEStatefulsetDeleteFailed"
	FEServiceDeleteFailed             EventReason = "FEServiceDeleteFailed"
	ComputeGroupsEmpty                EventReason = "CGsEmpty"
	CGUniqueIdentifierDuplicate       EventReason = "CGUniqueIdentifierDuplicate"
	CGUniqueIdentifierNotMatchRegex   EventReason = "CGUniqueIdentifierNotMatchRegex"
	CGCreateResourceFailed            EventReason = "CGCreateResourceFailed"
	CGApplyResourceFailed             EventReason = "CGApplyResourceFailed"
	CGStatusUpdateFailed              EventReason = "CGStatusUpdatedFailed"
	CGStatefulsetDeleteFailed         EventReason = "CGStatefulsetDeleteFailed"
	CGServiceDeleteFailed             EventReason = "CGServiceDeleteFailed"
	DisaggregatedMetaServiceGetFailed EventReason = "DisaggregatedMetaServiceGetFailed"
	ObjectInfoInvalid                 EventReason = "ObjectInfoInvalid"
	ConfigMapGetFailed                EventReason = "ConfigMapGetFailed"
	ObjectConfigError                 EventReason = "ObjectConfigError"
	MSInteractError                   EventReason = "MSInteractError"
	InstanceMetaCreated               EventReason = "InstanceMetaCreated"
	InstanceIdModified                EventReason = "InstanceIdModified"
	ConfigMapPathRepeated             EventReason = "ConfigMapPathRepeated"
)

type Event struct {
	Type    EventType
	Reason  EventReason
	Message string
}

func EventString(event *Event) string {
	return string(event.Reason) + "," + event.Message
}
