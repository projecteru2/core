package simplescheduler

import "fmt"
import "github.com/projecteru2/core/types"

// Add CPUMap b back to CPUMap c
func addCPUMap(c, b types.CPUMap) {
	for label, value := range b {
		if _, ok := c[label]; !ok {
			c[label] = value
		} else {
			c[label] += value
		}
	}
}

// Get quota from CPUMap c
// Returns the corresponding CPUMap
// If quota is 0, just return empty CPUMap
func getQuota(c types.CPUMap, quota int) (types.CPUMap, error) {
	r := types.CPUMap{}
	if quota == 0 {
		return r, nil
	}

	if cpuCount(c) < quota {
		return r, fmt.Errorf("Can't get quota, not enough resources")
	}

	// get quota num of labels
	// take the whole value from label
	for i := 0; i < quota; i++ {
		for label, value := range c {
			if value > 0 {
				r[label] = value
				break
			}
		}
	}
	// label which is taken away is set to 0 now
	// that's why we call this simple scheduler
	for label, _ := range r {
		c[label] = 0
	}
	return r, nil
}

func cpuCount(c types.CPUMap) int {
	count := 0
	for _, value := range c {
		if value > 0 {
			count += 1
		}
	}
	return count
}
