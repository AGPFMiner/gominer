package statistics

type HashRate struct {
	dataSeries [3600]float64
	currentPos int
}

func (hr *HashRate) Add(num float64) {
	hr.currentPos = (hr.currentPos + 1) % 3600
	hr.dataSeries[hr.currentPos] = num
}

func (hr *HashRate) RecentNSum(recentn int) (sum float64) {
	sum = 0
	pos := 0
	for i := 0; i < recentn; i++ {
		pos = (hr.currentPos - i)
		if pos < 0 {
			pos += 3600
		}
		sum += hr.dataSeries[pos]
	}
	return
}
