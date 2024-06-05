package mysql

// DecommissionInfo Decommission task info
type DecommissionInfo struct {
	SuccessBes         []*Backend
	DecommissioningBes []*Backend
}

func (di *DecommissionInfo) IsFinished() bool {
	if len(di.DecommissioningBes) > 0 {
		return false
	}
	return true
}

// NewDecommissionInfo build DecommissionInfo check successes nodes and dropping nodes
// currentNodes is show backends res, decommissionBes is decommissioned target be nodes
func NewDecommissionInfo(currentNodes []*Backend, decommissionBes []*Backend) *DecommissionInfo {
	var successBes []*Backend
	var decommissioningBes []*Backend
	decommissioningMap := make(map[string]*Backend)

	for _, node := range currentNodes {
		if node.SystemDecommissioned {
			decommissioningBes = append(decommissioningBes, node)
			decommissioningMap[node.Host] = node
		}
	}

	for _, be := range decommissionBes {
		_, ok := decommissioningMap[be.Host]
		if !ok {
			successBes = append(successBes, be)
		}
	}
	//fmt.Printf("finished:%d , droping: %d  ", len(successBes), len(decommissioningBes))
	return &DecommissionInfo{successBes, decommissioningBes}

}
