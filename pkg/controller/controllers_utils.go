package controller

import (
	"github.com/selectdb/doris-operator/api/doris/v1"
	"sort"
)

func inconsistentStatus(status *v1.DorisClusterStatus, dcr *v1.DorisCluster) bool {
	return inconsistentFEStatus(status.FEStatus, dcr.Status.FEStatus) ||
		inconsistentBEStatus(status.BEStatus, dcr.Status.BEStatus) ||
		inconsistentCnStatus(status.CnStatus, dcr.Status.CnStatus) ||
		inconsistentBrokerStatus(status.BrokerStatus, dcr.Status.BrokerStatus)
}

func inconsistentCnStatus(eStatus *v1.CnStatus, nStatus *v1.CnStatus) bool {
	if eStatus == nil && nStatus == nil {
		return false
	}

	eComponentStatus := v1.ComponentStatus{}
	nComponentStatus := v1.ComponentStatus{}
	var eHorizontalScaler, nHorizontalScaler *v1.HorizontalScaler
	if eStatus != nil {
		eComponentStatus = eStatus.ComponentStatus
		eHorizontalScaler = eStatus.HorizontalScaler
	}
	if nStatus != nil {
		nComponentStatus = nStatus.ComponentStatus
		nHorizontalScaler = nStatus.HorizontalScaler
	}

	return inconsistentComponentStatus(&eComponentStatus, &nComponentStatus) || inconsistentHorizontalStatus(eHorizontalScaler, nHorizontalScaler)
}

func inconsistentFEStatus(eFeStatus *v1.ComponentStatus, nFeStatus *v1.ComponentStatus) bool {
	return inconsistentComponentStatus(eFeStatus, nFeStatus)
}

func inconsistentBEStatus(eBeStatus *v1.ComponentStatus, nBeStatus *v1.ComponentStatus) bool {
	return inconsistentComponentStatus(eBeStatus, nBeStatus)
}

func inconsistentBrokerStatus(eBkStatus *v1.ComponentStatus, nBkStatus *v1.ComponentStatus) bool {
	return inconsistentComponentStatus(eBkStatus, nBkStatus)
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
