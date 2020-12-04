package types

// GetCapacity .
func GetCapacity(scheduleInfos []ScheduleInfo) map[string]int {
	capacity := make(map[string]int)
	for _, scheduleInfo := range scheduleInfos {
		capacity[scheduleInfo.Name] = scheduleInfo.Capacity
	}
	return capacity
}
