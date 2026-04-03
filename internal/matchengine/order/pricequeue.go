package order

type PriceQueue struct {
	Orders []*Order
}

func (pq *PriceQueue) Len() int {
	return len(pq.Orders)
}

func (pq *PriceQueue) Less(i, j int) bool {
	return pq.Orders[i].Price.Cmp(pq.Orders[j].Price) > 0
}

func (pq *PriceQueue) Swap(i, j int) {
	pq.Orders[i], pq.Orders[j] = pq.Orders[j], pq.Orders[i]
}

func (pq *PriceQueue) Push(x interface{}) {
	pq.Orders = append(pq.Orders, x.(*Order))
}

func (pq *PriceQueue) Pop() interface{} {
	old := pq.Orders
	n := len(old)
	item := old[n-1]
	pq.Orders = old[0 : n-1]
	return item
}

func NewPriceQueue() *PriceQueue {
	return &PriceQueue{
		Orders: make([]*Order, 0),
	}
}
