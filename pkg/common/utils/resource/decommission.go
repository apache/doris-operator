package resource

import (
	"github.com/apache/doris-operator/pkg/common/utils/mysql"
)

type DecommissionPhase string

const (
	Decommissioned           DecommissionPhase = "Decommissioned"
	Decommissioning          DecommissionPhase = "Decommissioning"
	DecommissionPhaseSteady  DecommissionPhase = "Steady"
	DecommissionPhaseUnknown DecommissionPhase = "Unknown"
)

type DecommissionDetail struct {
	AllBackendsSize       int
	UnDecommissionedCount int
	DecommissioningCount  int
	DecommissionedCount   int
	BeKeepAmount          int
}

func ConstructDecommissionDetail(allBackends []*mysql.Backend, cgKeepAmount int32) DecommissionDetail {
	var unDecommissionedCount, decommissioningCount, decommissionedCount int

	for i := range allBackends {
		node := allBackends[i]
		if !node.SystemDecommissioned {
			unDecommissionedCount++
		} else {
			if node.TabletNum == 0 {
				decommissionedCount++
			} else {
				decommissioningCount++
			}
		}
	}

	return DecommissionDetail{
		AllBackendsSize:       len(allBackends),
		UnDecommissionedCount: unDecommissionedCount,
		DecommissioningCount:  decommissioningCount,
		DecommissionedCount:   decommissionedCount,
		BeKeepAmount:          int(cgKeepAmount),
	}
}

func (d *DecommissionDetail) GetDecommissionDetailStatus() DecommissionPhase {
	if d.DecommissioningCount == 0 && d.DecommissionedCount == 0 && d.UnDecommissionedCount > d.BeKeepAmount {
		return DecommissionPhaseSteady
	}
	if d.UnDecommissionedCount == d.BeKeepAmount && d.DecommissioningCount > 0 {
		return Decommissioning
	}

	if d.UnDecommissionedCount == d.BeKeepAmount && d.UnDecommissionedCount+d.DecommissionedCount == d.AllBackendsSize {
		return Decommissioned
	}
	return DecommissionPhaseUnknown
}
