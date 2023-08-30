package controller

import (
	"github.com/selectdb/doris-operator/api/doris/v1"
	"sort"
)

func inconsistentStatus(status *v1.DorisClusterStatus, dcr *v1.DorisCluster) bool {
	return inconsistentComponentStatus(status.FEStatus, dcr.Status.FEStatus) ||
		inconsistentComponentStatus(status.BEStatus, dcr.Status.BEStatus) ||
		inconsistentComponentStatus(&status.CnStatus.ComponentStatus, &dcr.Status.CnStatus.ComponentStatus) ||
		inconsistentHorizontalStatus(status.CnStatus.HorizontalScaler, dcr.Status.CnStatus.HorizontalScaler)
}

func inconsistentComponentStatus(eStatus *v1.ComponentStatus, nStatus *v1.ComponentStatus) bool {
	if eStatus == nil && nStatus == nil {
		return false
	}

	// check resource status, if status not equal return true.
	if (eStatus == nil || nStatus == nil) ||
		eStatus.ComponentCondition != nStatus.ComponentCondition ||
		eStatus.AccessService != nStatus.AccessService {
		return true
	}

	//check control pods equal or not, if not return true.
	if !equalSplice(eStatus.CreatingMembers, nStatus.CreatingMembers) ||
		!equalSplice(eStatus.RunningMembers, nStatus.RunningMembers) ||
		!equalSplice(eStatus.FailedMembers, nStatus.FailedMembers) {
		return true
	}

	return false
}

func inconsistentHorizontalStatus(eh *v1.HorizontalScaler, nh *v1.HorizontalScaler) bool {
	if eh != nil && nh != nil {
		return eh.Name != nh.Name || eh.Version != nh.Version
	}

	if eh == nil && nh == nil {
		return false
	}
	return true
}

func equalSplice(e []string, n []string) bool {
	if len(e) != len(n) {
		return false
	}

	sort.Strings(e)
	sort.Strings(n)
	for i, _ := range e {
		if e[i] != n[i] {
			return false
		}
	}

	return true
}
