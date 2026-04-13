package streaming

import (
	"sync"

	apiv1 "github.com/shaminabd/ap2-contracts-go/apiv1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Hub struct {
	mu   sync.Mutex
	subs map[string][]chan *apiv1.OrderStatusUpdate
}

func NewHub() *Hub {
	return &Hub{subs: make(map[string][]chan *apiv1.OrderStatusUpdate)}
}

func (h *Hub) Register(orderID string) (updates <-chan *apiv1.OrderStatusUpdate, unregister func()) {
	ch := make(chan *apiv1.OrderStatusUpdate, 16)
	h.mu.Lock()
	h.subs[orderID] = append(h.subs[orderID], ch)
	h.mu.Unlock()
	return ch, func() {
		h.remove(orderID, ch)
	}
}

func (h *Hub) remove(orderID string, ch chan *apiv1.OrderStatusUpdate) {
	h.mu.Lock()
	defer h.mu.Unlock()
	list := h.subs[orderID]
	for i, c := range list {
		if c == ch {
			h.subs[orderID] = append(list[:i], list[i+1:]...)
			break
		}
	}
}

func (h *Hub) Notify(orderID, status string) {
	msg := &apiv1.OrderStatusUpdate{
		OrderId:   orderID,
		Status:    status,
		UpdatedAt: timestamppb.Now(),
	}
	h.mu.Lock()
	targets := append([]chan *apiv1.OrderStatusUpdate(nil), h.subs[orderID]...)
	h.mu.Unlock()
	for _, ch := range targets {
		select {
		case ch <- msg:
		default:
		}
	}
}
