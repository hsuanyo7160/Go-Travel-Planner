package main

import "time"

// ========== 輔助函數 ==========
func expandDays(startDate string, days int) []Day {
	result := make([]Day, days)
	start, _ := time.Parse("2006-01-02", startDate)

	for i := 0; i < days; i++ {
		date := start.AddDate(0, 0, i)
		result[i] = Day{
			DayIndex: i + 1,
			Date:     date.Format("2006-01-02"),
			Items:    []Item{},
		}
	}

	return result
}
