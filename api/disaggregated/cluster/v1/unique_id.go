package v1

import (
	"strings"
)

/*
please use get function to replace new function.
*/

func newCGStatefulsetName(ddcName /*dorisDisaggregatedCluster Name*/, cgName /*computegroup's name*/ string) string {
	return ddcName + "-" + cgName
}

// RE:[a-zA-Z][0-9a-zA-Z_]+
func newCGClusterId(namespace, stsName string) string {
	return strings.ReplaceAll(namespace+"_"+stsName, "-", "_")
}

// RE:[a-zA-Z][0-9a-zA-Z_]+
func newCGCloudUniqueId(namespace, instanceName, statefulsetName string) string {
	return strings.ReplaceAll("1:"+namespace+"_"+instanceName+":"+statefulsetName, "-", "_")
}

func (ddc *DorisDisaggregatedCluster) GetCGStatefulsetName(cg *ComputeGroup) string {
	cgStsName := ""
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if cgs.ComputeGroupName == cg.Name || cgs.ClusterId == cg.ClusterId || cgs.CloudUniqueId == cg.CloudUniqueId {
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

	// need config in CR.
	return ""
}
func (ddc *DorisDisaggregatedCluster) GetCGClusterId(cg *ComputeGroup) string {
	if cg == nil || ddc == nil {
		return ""
	}
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if cg.Name == cgs.ComputeGroupName || cg.ClusterId == cgs.ClusterId || cg.CloudUniqueId == cgs.CloudUniqueId {
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

func (ddc *DorisDisaggregatedCluster) GetCGCloudUniqueId(cg *ComputeGroup) string {
	if cg == nil || ddc == nil {
		return ""
	}
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if cg.Name == cgs.ComputeGroupName || cg.ClusterId == cgs.ClusterId || cg.CloudUniqueId == cgs.CloudUniqueId {
			return cg.CloudUniqueId
		}
	}

	statefulsetName := ddc.GetCGStatefulsetName(cg)
	//update cg' clusterId for auto assemble, if not config.
	if cg.CloudUniqueId == "" {
		cg.CloudUniqueId = newCGCloudUniqueId(ddc.Namespace, ddc.Name, statefulsetName)
	}

	return cg.CloudUniqueId
}

func (ddc *DorisDisaggregatedCluster) GetFEStatefulsetName() string {
	return ddc.Name + "-" + "fe"
}

func (ddc *DorisDisaggregatedCluster) GetCGServiceName(cg *ComputeGroup) string {
	svcName := ""
	for _, cgs := range ddc.Status.ComputeGroupStatuses {
		if cgs.ComputeGroupName == cg.Name || cgs.ClusterId == cg.ClusterId || cgs.CloudUniqueId == cg.CloudUniqueId {
			svcName = cgs.ServiceName
		}
	}

	if svcName != "" {
		return svcName
	}

	return ddc.Name + "-" + cg.Name
}

func (ddc *DorisDisaggregatedCluster) GetFEServiceName() string {
	return ddc.Name + "-" + "fe"
}
