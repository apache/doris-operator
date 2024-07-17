package v1

import (
	"fmt"
	"strings"
)

/*
please use get function to replace new function.
*/

func newCGStatefulsetName(ddcName /*dorisDisaggregatedCluster Name*/, cgName /*computegroup's name*/ string) string {
	//cgName use "_", but name in kubernetes object use "-"
	stName := ddcName + "-" + cgName
	stName = strings.ReplaceAll(stName, "_", "-")
	return stName
}

// RE:[a-zA-Z][0-9a-zA-Z_]+
func newCGClusterId(namespace, stsName string) string {
	return strings.ReplaceAll(namespace+"_"+stsName, "-", "_")
}

func newCGCloudUniqueIdPre(instanceId string) string {
	return fmt.Sprintf("1:%s", instanceId)
}

func (ddc *DorisDisaggregatedCluster) GetCGStatefulsetName(cg *ComputeGroup) string {
	cgStsName := ""
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if cgs.ComputeGroupName == cg.Name || cgs.ClusterId == cg.ClusterId {
			cgStsName = cgs.StatefulsetName
		}
	}

	if cgStsName != "" {
		return cgStsName
	}
	return newCGStatefulsetName(ddc.Name, cg.Name)
}

func (ddc *DorisDisaggregatedCluster) GetInstanceId() string {
	if ddc.Status.InstanceId != "" {
		return ddc.Status.InstanceId
	}

	// need config in vaultConfigMap.
	return ""
}
func (ddc *DorisDisaggregatedCluster) GetCGClusterId(cg *ComputeGroup) string {
	if cg == nil || ddc == nil {
		return ""
	}
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if cg.Name == cgs.ComputeGroupName || cg.ClusterId == cgs.ClusterId {
			return cg.ClusterId
		}
	}

	stsName := ddc.GetCGStatefulsetName(cg)
	//update cg' clusterId for auto assemble, if not config.
	if cg.ClusterId == "" {
		cg.ClusterId = newCGClusterId(ddc.Namespace, stsName)
	}

	return cg.ClusterId
}

func (ddc *DorisDisaggregatedCluster) GetCGCloudUniqueIdPre() string {
	return newCGCloudUniqueIdPre(ddc.GetInstanceId())
}

func (ddc *DorisDisaggregatedCluster) GetFEStatefulsetName() string {
	return ddc.Name + "-" + "fe"
}

func (ddc *DorisDisaggregatedCluster) GetCGServiceName(cg *ComputeGroup) string {
	svcName := ""
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if cgs.ComputeGroupName == cg.Name || cgs.ClusterId == cg.ClusterId {
			svcName = cgs.ServiceName
		}
	}

	if svcName != "" {
		return svcName
	}

	svcName = ddc.Name + "-" + cg.Name
	svcName = strings.ReplaceAll(svcName, "_", "-")
	return svcName
}

func (ddc *DorisDisaggregatedCluster) GetFEServiceName() string {
	return ddc.Name + "-" + "fe"
}
