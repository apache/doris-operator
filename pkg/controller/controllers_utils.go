package controller

import (
	"github.com/selectdb/doris-operator/api/doris/v1"
	"sort"
)

func inconsistentStatus(status *v1.DorisClusterStatus, dcr *v1.DorisCluster) bool {
	return inconsistentComponentStatus(status.FEStatus, dcr.Status.FEStatus) ||
		inconsistentComponentStatus(status.BEStatus, dcr.Status.BEStatus) ||
		inconsistentComponentStatus(status.CnStatus, dcr.Status.CnStatus)
}

func inconsistentComponentStatus(eStatus *v1.ComponentStatus, nStatus *v1.ComponentStatus) bool {
	if eStatus == nil && nStatus == nil {
		return false
	}

	if (eStatus == nil || nStatus == nil) ||
		eStatus.ComponentCondition != nStatus.ComponentCondition ||
		eStatus.AccessService != nStatus.AccessService {
		return true
	}

	if !equalSplice(eStatus.CreatingMembers, nStatus.CreatingMembers) ||
		!equalSplice(eStatus.RunningMembers, nStatus.RunningMembers) ||
		!equalSplice(eStatus.FailedMembers, nStatus.FailedMembers) {
		return true
	}

	return false
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
