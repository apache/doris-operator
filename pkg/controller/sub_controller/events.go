// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

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
	FEHTTPFailed                      EventReason = "FEHTTPResponseFailed"
	FEServiceDeleteFailed             EventReason = "FEServiceDeleteFailed"
	FEStatusUpdateFailed              EventReason = "FEStatusUpdatedFailed"
	ComputeClustersEmpty              EventReason = "CCsEmpty"
	CCUniqueIdentifierDuplicate       EventReason = "CCUniqueIdentifierDuplicate"
	CCUniqueIdentifierNotMatchRegex   EventReason = "CCUniqueIdentifierNotMatchRegex"
	CCCreateResourceFailed            EventReason = "CCCreateResourceFailed"
	CCApplyResourceFailed             EventReason = "CCApplyResourceFailed"
	CCStatusUpdateFailed              EventReason = "CCStatusUpdatedFailed"
	CCStatefulsetDeleteFailed         EventReason = "CCStatefulsetDeleteFailed"
	CCServiceDeleteFailed             EventReason = "CCServiceDeleteFailed"
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
